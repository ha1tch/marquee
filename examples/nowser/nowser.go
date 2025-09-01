///////////////////////////////////////
// The Nowser Antiexplorer           //
// Powered by MARQUEE and Bloatscape //
// --------------------------------- //
// Version 0.6 - Resource Caching    //
///////////////////////////////////////

package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/marquee"
)

// ResourceType defines the type of cached resource
type ResourceType int

const (
	ResourceTypePage ResourceType = iota
	ResourceTypeCSS
	ResourceTypeImage
	ResourceTypeFont
)

// CacheEntry represents a single cached resource
type CacheEntry struct {
	Key         string            `json:"key"`
	URL         string            `json:"url"`
	Data        []byte            `json:"data"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers"`
	CachedAt    time.Time         `json:"cached_at"`
	AccessedAt  time.Time         `json:"accessed_at"`
	Size        int64             `json:"size"`
	Type        ResourceType      `json:"type"`
}

// LRUNode represents a node in the LRU linked list
type LRUNode struct {
	Key  string
	Next *LRUNode
	Prev *LRUNode
}

// LRUList manages least recently used eviction
type LRUList struct {
	head *LRUNode
	tail *LRUNode
	size int
}

// NewLRUList creates a new LRU list
func NewLRUList() *LRUList {
	head := &LRUNode{}
	tail := &LRUNode{}
	head.Next = tail
	tail.Prev = head
	return &LRUList{head: head, tail: tail}
}

// Add adds a key to the front of the list
func (lru *LRUList) Add(key string) *LRUNode {
	node := &LRUNode{Key: key}
	node.Next = lru.head.Next
	node.Prev = lru.head
	lru.head.Next.Prev = node
	lru.head.Next = node
	lru.size++
	return node
}

// Remove removes a node from the list
func (lru *LRUList) Remove(node *LRUNode) {
	node.Prev.Next = node.Next
	node.Next.Prev = node.Prev
	lru.size--
}

// RemoveTail removes and returns the least recently used key
func (lru *LRUList) RemoveTail() string {
	if lru.size == 0 {
		return ""
	}
	node := lru.tail.Prev
	lru.Remove(node)
	return node.Key
}

// MoveToFront moves an existing node to the front
func (lru *LRUList) MoveToFront(node *LRUNode) {
	lru.Remove(node)
	node.Next = lru.head.Next
	node.Prev = lru.head
	lru.head.Next.Prev = node
	lru.head.Next = node
	lru.size++
}

// ResourceCache manages cached resources with LRU eviction
type ResourceCache struct {
	entries     map[string]*CacheEntry
	lruNodes    map[string]*LRUNode
	lru         *LRUList
	maxSize     int64
	maxEntries  int
	currentSize int64
	mutex       sync.RWMutex
	cacheDir    string
}

// NewResourceCache creates a new resource cache
func NewResourceCache(cacheDir string, maxSize int64, maxEntries int) *ResourceCache {
	return &ResourceCache{
		entries:    make(map[string]*CacheEntry),
		lruNodes:   make(map[string]*LRUNode),
		lru:        NewLRUList(),
		maxSize:    maxSize,
		maxEntries: maxEntries,
		cacheDir:   cacheDir,
	}
}

// generateKey creates a cache key from URL
func (rc *ResourceCache) generateKey(url string) string {
	hash := md5.Sum([]byte(url))
	return fmt.Sprintf("%x", hash)
}

// Get retrieves a resource from cache
func (rc *ResourceCache) Get(url string) (*CacheEntry, bool) {
	key := rc.generateKey(url)

	rc.mutex.RLock()
	entry, exists := rc.entries[key]
	rc.mutex.RUnlock()

	if !exists {
		return nil, false
	}

	// Update access time and LRU position
	rc.mutex.Lock()
	entry.AccessedAt = time.Now()
	if node, nodeExists := rc.lruNodes[key]; nodeExists {
		rc.lru.MoveToFront(node)
	}
	rc.mutex.Unlock()

	return entry, true
}

// Store adds a resource to cache
func (rc *ResourceCache) Store(url string, data []byte, contentType string, headers map[string]string, resourceType ResourceType) error {
	key := rc.generateKey(url)
	size := int64(len(data))

	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	// Check if we need to evict entries
	for (rc.currentSize+size > rc.maxSize || len(rc.entries) >= rc.maxEntries) && rc.lru.size > 0 {
		rc.evictLRU()
	}

	// Create new entry
	entry := &CacheEntry{
		Key:         key,
		URL:         url,
		Data:        data,
		ContentType: contentType,
		Headers:     headers,
		CachedAt:    time.Now(),
		AccessedAt:  time.Now(),
		Size:        size,
		Type:        resourceType,
	}

	// Remove existing entry if present
	if oldEntry, exists := rc.entries[key]; exists {
		rc.currentSize -= oldEntry.Size
		if node, nodeExists := rc.lruNodes[key]; nodeExists {
			rc.lru.Remove(node)
			delete(rc.lruNodes, key)
		}
	}

	// Add new entry
	rc.entries[key] = entry
	rc.lruNodes[key] = rc.lru.Add(key)
	rc.currentSize += size

	return nil
}

// evictLRU removes the least recently used entry
func (rc *ResourceCache) evictLRU() {
	key := rc.lru.RemoveTail()
	if key == "" {
		return
	}

	if entry, exists := rc.entries[key]; exists {
		rc.currentSize -= entry.Size
		delete(rc.entries, key)
		delete(rc.lruNodes, key)
	}
}

// LoadFromDisk loads cache from filesystem
func (rc *ResourceCache) LoadFromDisk() error {
	indexFile := filepath.Join(rc.cacheDir, "index.json")

	data, err := os.ReadFile(indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file exists yet, not an error
		}
		return err
	}

	var entries []*CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	for _, entry := range entries {
		// Load data from separate file
		dataFile := filepath.Join(rc.cacheDir, entry.Key+".dat")
		data, err := os.ReadFile(dataFile)
		if err != nil {
			continue // Skip corrupted entries
		}

		entry.Data = data
		entry.Size = int64(len(data))

		// Only load if not expired (simple 24h expiry for now)
		if time.Since(entry.CachedAt) < 24*time.Hour {
			rc.entries[entry.Key] = entry
			rc.lruNodes[entry.Key] = rc.lru.Add(entry.Key)
			rc.currentSize += entry.Size
		}
	}

	return nil
}

// SaveToDisk persists cache to filesystem
func (rc *ResourceCache) SaveToDisk() error {
	if err := os.MkdirAll(rc.cacheDir, 0755); err != nil {
		return err
	}

	rc.mutex.RLock()

	// Create index of all entries (without data)
	var entries []*CacheEntry
	for _, entry := range rc.entries {
		// Create copy without data for index
		indexEntry := &CacheEntry{
			Key:         entry.Key,
			URL:         entry.URL,
			ContentType: entry.ContentType,
			Headers:     entry.Headers,
			CachedAt:    entry.CachedAt,
			AccessedAt:  entry.AccessedAt,
			Size:        entry.Size,
			Type:        entry.Type,
		}
		entries = append(entries, indexEntry)

		// Save data to separate file
		dataFile := filepath.Join(rc.cacheDir, entry.Key+".dat")
		os.WriteFile(dataFile, entry.Data, 0644)
	}

	rc.mutex.RUnlock()

	// Save index
	indexData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	indexFile := filepath.Join(rc.cacheDir, "index.json")
	return os.WriteFile(indexFile, indexData, 0644)
}

// Clear removes all entries from cache
func (rc *ResourceCache) Clear() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	rc.entries = make(map[string]*CacheEntry)
	rc.lruNodes = make(map[string]*LRUNode)
	rc.lru = NewLRUList()
	rc.currentSize = 0
}

// ResourceCacheManager manages multiple resource caches
type ResourceCacheManager struct {
	caches  map[ResourceType]*ResourceCache
	baseDir string
	mutex   sync.RWMutex
}

// NewResourceCacheManager creates a new resource cache manager
func NewResourceCacheManager() *ResourceCacheManager {
	homeDir, _ := os.UserHomeDir()
	baseDir := filepath.Join(homeDir, ".nowser", "cache")

	rcm := &ResourceCacheManager{
		caches:  make(map[ResourceType]*ResourceCache),
		baseDir: baseDir,
	}

	// Initialize caches for different resource types
	rcm.caches[ResourceTypePage] = NewResourceCache(
		filepath.Join(baseDir, "pages"),
		100*1024*1024, // 100MB
		2000)          // 2000 entries

	rcm.caches[ResourceTypeCSS] = NewResourceCache(
		filepath.Join(baseDir, "css"),
		50*1024*1024, // 50MB
		1000)         // 1000 entries

	rcm.caches[ResourceTypeImage] = NewResourceCache(
		filepath.Join(baseDir, "images"),
		200*1024*1024, // 200MB
		5000)          // 5000 entries

	rcm.caches[ResourceTypeFont] = NewResourceCache(
		filepath.Join(baseDir, "fonts"),
		20*1024*1024, // 20MB
		100)          // 100 entries

	return rcm
}

// Get retrieves a resource from the appropriate cache
func (rcm *ResourceCacheManager) Get(url string, resourceType ResourceType) (*CacheEntry, bool) {
	rcm.mutex.RLock()
	cache, exists := rcm.caches[resourceType]
	rcm.mutex.RUnlock()

	if !exists {
		return nil, false
	}

	return cache.Get(url)
}

// Store adds a resource to the appropriate cache
func (rcm *ResourceCacheManager) Store(url string, data []byte, contentType string, headers map[string]string, resourceType ResourceType) error {
	rcm.mutex.RLock()
	cache, exists := rcm.caches[resourceType]
	rcm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("no cache for resource type %d", resourceType)
	}

	return cache.Store(url, data, contentType, headers, resourceType)
}

// LoadFromDisk loads all caches from filesystem
func (rcm *ResourceCacheManager) LoadFromDisk() error {
	rcm.mutex.RLock()
	defer rcm.mutex.RUnlock()

	for _, cache := range rcm.caches {
		if err := cache.LoadFromDisk(); err != nil {
			// Don't fail completely, just log the error
			fmt.Printf("Warning: Failed to load cache: %v\n", err)
		}
	}

	return nil
}

// SaveToDisk persists all caches to filesystem
func (rcm *ResourceCacheManager) SaveToDisk() error {
	rcm.mutex.RLock()
	defer rcm.mutex.RUnlock()

	for _, cache := range rcm.caches {
		if err := cache.SaveToDisk(); err != nil {
			fmt.Printf("Warning: Failed to save cache: %v\n", err)
		}
	}

	return nil
}

// GetCacheStats returns cache statistics
func (rcm *ResourceCacheManager) GetCacheStats() map[ResourceType]string {
	stats := make(map[ResourceType]string)

	rcm.mutex.RLock()
	defer rcm.mutex.RUnlock()

	for resourceType, cache := range rcm.caches {
		cache.mutex.RLock()
		stats[resourceType] = fmt.Sprintf("%d entries, %.1f MB",
			len(cache.entries),
			float64(cache.currentSize)/(1024*1024))
		cache.mutex.RUnlock()
	}

	return stats
}

// TabSession represents saved tab state
type TabSession struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	ScrollY float32 `json:"scroll_y"`
}

// BrowserSession represents the entire browser state for saving
type BrowserSession struct {
	Tabs           []TabSession `json:"tabs"`
	ActiveTabIndex int          `json:"active_tab_index"`
}

// Tab represents a single browser tab
type Tab struct {
	Title         string
	URL           string
	AddressBar    string
	Widget        *marquee.HTMLWidget
	StatusMessage string
	Loading       bool
	LoadingStart  time.Time
	ScrollY       float32 // Preserve scroll position per tab
	PendingHTML   string  // HTML content waiting to be processed on main thread
	PendingTitle  string  // Title waiting to be processed on main thread
	PendingURL    string  // URL waiting to be processed on main thread
	HasPending    bool    // Flag to indicate pending content
}

// BrowserApp represents the main browser application
type BrowserApp struct {
	tabs             []*Tab
	activeTabIndex   int
	editingAddress   bool
	cursorPos        int
	history          []string
	historyIndex     int
	tabHeight        float32
	addressBarHeight float32
	statusBarHeight  float32
	isMac            bool
	modifierKey      string // "Cmd" or "Ctrl"
	// Chrome animation
	chromeVisible     bool
	chromeAnimating   bool
	animationStart    time.Time
	animationDuration time.Duration
	chromeOffset      float32 // Current offset for animation
	// Tab drag & drop
	draggingTab      bool
	dragTabIndex     int
	dragStartX       float32
	dragStartY       float32
	dragOffset       float32
	dragTargetIndex  int
	potentialDragTab int // Tab that might be dragged
	dragStartTime    time.Time
	dragThreshold    float32 // Minimum distance to start drag
	// Window dimensions
	windowWidth  float32
	windowHeight float32
	lastWidth    float32
	lastHeight   float32
	// NEW: Resource cache manager
	cacheManager *ResourceCacheManager
}

// NewBrowserApp creates a new browser application with one initial tab
func NewBrowserApp() *BrowserApp {
	isMac := runtime.GOOS == "darwin"
	modifierKey := "Ctrl"
	if isMac {
		modifierKey = "Cmd"
	}

	app := &BrowserApp{
		tabs:              make([]*Tab, 0),
		activeTabIndex:    0,
		tabHeight:         35,
		addressBarHeight:  40,
		statusBarHeight:   30,
		isMac:             isMac,
		modifierKey:       modifierKey,
		chromeVisible:     true,
		chromeAnimating:   false,
		animationDuration: 350 * time.Millisecond,
		chromeOffset:      0,
		draggingTab:       false,
		dragTabIndex:      -1,
		dragTargetIndex:   -1,
		potentialDragTab:  -1,
		dragThreshold:     8.0,
		windowWidth:       900,
		windowHeight:      700,
		lastWidth:         900,
		lastHeight:        700,
		// NEW: Initialize cache manager
		cacheManager: NewResourceCacheManager(),
	}

	// Load cache from disk
	if err := app.cacheManager.LoadFromDisk(); err != nil {
		fmt.Printf("Warning: Could not load cache: %v\n", err)
	}

	// Try to load saved session
	if !app.loadSession() {
		// No saved session, create initial tab
		startURL := "https://example.com"
		if _, err := os.Stat("index.html"); err == nil {
			pwd, _ := os.Getwd()
			startURL = "file://" + pwd + "/index.html"
		}
		app.createNewTab(startURL, true)
	}

	return app
}

// Update window dimensions and handle resize events
func (app *BrowserApp) updateWindowDimensions() {
	app.windowWidth = float32(rl.GetScreenWidth())
	app.windowHeight = float32(rl.GetScreenHeight())

	// Check if window was resized
	if app.windowWidth != app.lastWidth || app.windowHeight != app.lastHeight {
		app.onWindowResize()
		app.lastWidth = app.windowWidth
		app.lastHeight = app.windowHeight
	}
}

// Handle window resize event
func (app *BrowserApp) onWindowResize() {
	// If we have a very narrow window, hide chrome to maximize content space
	if app.windowWidth < 600 && app.chromeVisible {
		app.chromeVisible = false
		app.chromeOffset = app.getTotalChromeHeight()
	}

	// Clamp minimum window size for usability
	minWidth := float32(300)
	minHeight := float32(200)

	if app.windowWidth < minWidth {
		app.windowWidth = minWidth
	}
	if app.windowHeight < minHeight {
		app.windowHeight = minHeight
	}

	// Update tab drag calculations if currently dragging
	if app.draggingTab {
		// Recalculate drag target based on new window width
		tabWidth := app.getTabWidth()
		mousePos := rl.GetMousePosition()
		app.dragTargetIndex = int(mousePos.X / tabWidth)

		if app.dragTargetIndex < 0 {
			app.dragTargetIndex = 0
		}
		if app.dragTargetIndex >= len(app.tabs) {
			app.dragTargetIndex = len(app.tabs) - 1
		}
	}
}

// Get total chrome height
func (app *BrowserApp) getTotalChromeHeight() float32 {
	return app.tabHeight + app.addressBarHeight + app.statusBarHeight
}

// Get dynamic tab width based on window size
func (app *BrowserApp) getTabWidth() float32 {
	if len(app.tabs) == 0 {
		return 200
	}

	// Reserve space for new tab button
	availableWidth := app.windowWidth - 30
	tabWidth := availableWidth / float32(len(app.tabs))

	// Set reasonable min and max tab widths
	minTabWidth := float32(80)
	maxTabWidth := float32(250)

	if tabWidth < minTabWidth {
		tabWidth = minTabWidth
	} else if tabWidth > maxTabWidth {
		tabWidth = maxTabWidth
	}

	return tabWidth
}

// Get content area dimensions
func (app *BrowserApp) getContentArea() (x, y, width, height float32) {
	x = 20
	y = app.tabHeight + app.addressBarHeight - app.chromeOffset + 10
	width = app.windowWidth - 40 // 20px margin on each side
	height = app.windowHeight - y - app.statusBarHeight - 10 + app.chromeOffset

	// Ensure minimum content area
	if width < 100 {
		width = 100
	}
	if height < 100 {
		height = 100
	}

	return x, y, width, height
}

// Get .nowser directory path
func getNowserDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".nowser"
	}
	return filepath.Join(home, ".nowser")
}

// Save browser session
func (app *BrowserApp) saveSession() {
	nowserDir := getNowserDir()
	os.MkdirAll(nowserDir, 0755)

	session := BrowserSession{
		Tabs:           make([]TabSession, 0),
		ActiveTabIndex: app.activeTabIndex,
	}

	for _, tab := range app.tabs {
		if tab.URL != "" {
			scrollY := float32(0)
			if tab.Widget != nil {
				scrollY = tab.Widget.ScrollY
			}

			session.Tabs = append(session.Tabs, TabSession{
				Title:   tab.Title,
				URL:     tab.URL,
				ScrollY: scrollY,
			})
		}
	}

	if len(session.Tabs) == 0 {
		return
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return
	}

	sessionFile := filepath.Join(nowserDir, "tabs.json")
	os.WriteFile(sessionFile, data, 0644)
}

// Load browser session
func (app *BrowserApp) loadSession() bool {
	nowserDir := getNowserDir()
	sessionFile := filepath.Join(nowserDir, "tabs.json")

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return false
	}

	var session BrowserSession
	if err := json.Unmarshal(data, &session); err != nil {
		return false
	}

	if len(session.Tabs) == 0 {
		return false
	}

	for _, tabSession := range session.Tabs {
		tab := &Tab{
			Title:         tabSession.Title,
			URL:           tabSession.URL,
			AddressBar:    tabSession.URL,
			StatusMessage: "Restored from session",
			ScrollY:       tabSession.ScrollY,
		}
		app.tabs = append(app.tabs, tab)
	}

	if session.ActiveTabIndex >= 0 && session.ActiveTabIndex < len(app.tabs) {
		app.activeTabIndex = session.ActiveTabIndex
	}

	if app.activeTabIndex < len(app.tabs) {
		app.loadURL(app.tabs[app.activeTabIndex].URL)
	}

	return true
}

// Handle tab drag and drop
func (app *BrowserApp) handleTabDrag() {
	mousePos := rl.GetMousePosition()

	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && !app.draggingTab && app.potentialDragTab == -1 {
		tabWidth := app.getTabWidth()
		tabBarY := -app.chromeOffset
		currentX := float32(0)

		for i := range app.tabs {
			tabRect := rl.NewRectangle(currentX, tabBarY, tabWidth, app.tabHeight)

			if rl.CheckCollisionPointRec(mousePos, tabRect) {
				closeX := currentX + tabWidth - 20
				if mousePos.X < closeX {
					app.potentialDragTab = i
					app.dragStartX = mousePos.X
					app.dragStartY = mousePos.Y
					app.dragStartTime = time.Now()
					return
				}
			}
			currentX += tabWidth
		}
	}

	if app.potentialDragTab != -1 {
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			distance := float32(0)
			if mousePos.X > app.dragStartX {
				distance = mousePos.X - app.dragStartX
			} else {
				distance = app.dragStartX - mousePos.X
			}

			yDistance := float32(0)
			if mousePos.Y > app.dragStartY {
				yDistance = mousePos.Y - app.dragStartY
			} else {
				yDistance = app.dragStartY - mousePos.Y
			}

			totalDistance := distance + yDistance

			if totalDistance > app.dragThreshold {
				app.draggingTab = true
				app.dragTabIndex = app.potentialDragTab
				app.dragOffset = 0
				app.dragTargetIndex = app.potentialDragTab
				app.potentialDragTab = -1
			}
		} else {
			clickedTab := app.tabs[app.potentialDragTab]
			app.activeTabIndex = app.potentialDragTab
			app.editingAddress = false

			if clickedTab.Widget == nil && clickedTab.URL != "" {
				app.loadURL(clickedTab.URL)
			}

			app.potentialDragTab = -1
		}
	}

	if app.draggingTab {
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			app.dragOffset = mousePos.X - app.dragStartX

			tabWidth := app.getTabWidth()
			app.dragTargetIndex = int(mousePos.X / tabWidth)

			if app.dragTargetIndex < 0 {
				app.dragTargetIndex = 0
			}
			if app.dragTargetIndex >= len(app.tabs) {
				app.dragTargetIndex = len(app.tabs) - 1
			}
		} else {
			if app.dragTargetIndex != app.dragTabIndex {
				app.reorderTabs(app.dragTabIndex, app.dragTargetIndex)
			}

			app.draggingTab = false
			app.dragTabIndex = -1
			app.dragTargetIndex = -1
			app.dragOffset = 0
			app.saveSession()
		}
	}
}

// Reorder tabs by moving tab from oldIndex to newIndex
func (app *BrowserApp) reorderTabs(oldIndex, newIndex int) {
	if oldIndex == newIndex || oldIndex < 0 || newIndex < 0 ||
		oldIndex >= len(app.tabs) || newIndex >= len(app.tabs) {
		return
	}

	tab := app.tabs[oldIndex]
	app.tabs = append(app.tabs[:oldIndex], app.tabs[oldIndex+1:]...)
	app.tabs = append(app.tabs[:newIndex], append([]*Tab{tab}, app.tabs[newIndex:]...)...)

	if app.activeTabIndex == oldIndex {
		app.activeTabIndex = newIndex
	} else if app.activeTabIndex > oldIndex && app.activeTabIndex <= newIndex {
		app.activeTabIndex--
	} else if app.activeTabIndex < oldIndex && app.activeTabIndex >= newIndex {
		app.activeTabIndex++
	}
}

// Create a new tab
func (app *BrowserApp) createNewTab(initialURL string, loadImmediately bool) {
	tab := &Tab{
		Title:         "New Tab",
		URL:           "",
		AddressBar:    initialURL,
		StatusMessage: "Enter a URL and press Enter",
		Loading:       false,
	}

	if !loadImmediately {
		// Show cache stats in welcome message
		stats := app.cacheManager.GetCacheStats()
		cacheInfo := ""
		if pageStats, exists := stats[ResourceTypePage]; exists {
			cacheInfo = fmt.Sprintf("<p><i>Cache: %s</i></p>", pageStats)
		}

		welcomeHTML := fmt.Sprintf(`
			<h1>Nowser</h1>
			<p>A lightweight, <i>minimalist</i> browser built with <b>MARQUEE</b></p>
			%s
			<hr>
			<h2>Getting Started</h2>
			<ul>
				<li>Enter a URL in the address bar and press <b>Enter</b></li>
				<li>Use <b>%s+T</b> to open a new tab</li>
				<li>Use <b>%s+W</b> to close the current tab</li>
				<li>Use <b>%s+L</b> to focus the address bar</li>
				<li>Click on tabs to switch between them</li>
				<li><b>Middle click</b> anywhere to hide/show browser controls</li>
				<li><b>Drag & drop</b> HTML or TXT files to open them</li>
				<li><b>Resize</b> the window - everything scales automatically!</li>
			</ul>
			<h3>Philosophy</h3>
			<p>In an age of <i>bloated browsers</i>, Nowser embraces <b>unbloaticity</b>. No extensions, no trackers, no complexity - just pure browsing.</p>
			<p>Hide the browser chrome for <i>distraction-free reading</i> and focus on content.</p>
			<p>Try visiting: <a href="https://example.com">example.com</a> or <a href="https://www.berkshirehathaway.com">berkshirehathaway.com</a></p>
		`, cacheInfo, app.modifierKey, app.modifierKey, app.modifierKey)

		tab.Widget = marquee.NewHTMLWidget(welcomeHTML)
		tab.Title = "Welcome to Nowser"
		tab.setupLinkHandler(app)
	}

	app.tabs = append(app.tabs, tab)
	app.activeTabIndex = len(app.tabs) - 1

	if loadImmediately {
		app.loadURL(initialURL)
	}

	app.saveSession()
}

// Setup link click handler for a tab
func (tab *Tab) setupLinkHandler(app *BrowserApp) {
	if tab.Widget == nil {
		return
	}

	tab.Widget.OnLinkClick = func(clickedURL string) {
		if !strings.HasPrefix(clickedURL, "http://") && !strings.HasPrefix(clickedURL, "https://") && !strings.HasPrefix(clickedURL, "file://") {
			if tab.URL != "" {
				base, err := url.Parse(tab.URL)
				if err == nil {
					resolved, err := base.Parse(clickedURL)
					if err == nil {
						clickedURL = resolved.String()
					}
				}
			}
		}
		app.loadURL(clickedURL)
	}
}

// Close a tab
func (app *BrowserApp) closeTab(index int) {
	if len(app.tabs) <= 1 {
		return
	}

	if app.tabs[index].Widget != nil {
		app.tabs[index].Widget.Unload()
	}

	app.tabs = append(app.tabs[:index], app.tabs[index+1:]...)

	if app.activeTabIndex >= len(app.tabs) {
		app.activeTabIndex = len(app.tabs) - 1
	} else if app.activeTabIndex > index {
		app.activeTabIndex--
	}

	app.saveSession()
}

// Get current active tab
func (app *BrowserApp) currentTab() *Tab {
	if app.activeTabIndex >= 0 && app.activeTabIndex < len(app.tabs) {
		return app.tabs[app.activeTabIndex]
	}
	return nil
}

// Simple HTML sanitizer and converter
func sanitizeAndConvertHTML(html string) string {
	scriptRe := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")
	styleRe := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	unsupportedTags := []string{"div", "span", "section", "article", "nav", "header", "footer", "main", "aside", "meta", "link"}
	for _, tag := range unsupportedTags {
		openRe := regexp.MustCompile(fmt.Sprintf(`(?i)<%s[^>]*>`, tag))
		closeRe := regexp.MustCompile(fmt.Sprintf(`(?i)</%s>`, tag))
		html = openRe.ReplaceAllString(html, "")
		html = closeRe.ReplaceAllString(html, "")
	}

	entities := map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   "\"",
		"&nbsp;":   " ",
		"&mdash;":  "—", // EM DASH (long dash)
		"&ndash;":  "–", // EN DASH (medium dash)
		"&hellip;": "…", // HORIZONTAL ELLIPSIS (three dots)
	}

	for entity, replacement := range entities {
		html = strings.ReplaceAll(html, entity, replacement)
	}

	bodyRe := regexp.MustCompile(`(?i)<body[^>]*>(.*?)</body>`)
	if matches := bodyRe.FindStringSubmatch(html); len(matches) > 1 {
		html = matches[1]
	}

	return html
}

// Extract page title from HTML
func extractTitle(html string) string {
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	if matches := titleRe.FindStringSubmatch(html); len(matches) > 1 {
		title := strings.TrimSpace(matches[1])
		if title != "" {
			return title
		}
	}
	return "Untitled"
}

// NEW: Enhanced HTTP client with caching
func (app *BrowserApp) fetchWithCache(targetURL string) ([]byte, string, error) {
	// Check cache first
	if entry, found := app.cacheManager.Get(targetURL, ResourceTypePage); found {
		// Check if cache entry is still fresh (1 hour for pages)
		if time.Since(entry.CachedAt) < time.Hour {
			return entry.Data, entry.ContentType, nil
		}
	}

	// Not in cache or expired, fetch from network
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(targetURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Extract content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/html"
	}

	// Store in cache
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	app.cacheManager.Store(targetURL, body, contentType, headers, ResourceTypePage)

	return body, contentType, nil
}

// Load URL in current tab (SIGNATURE UNCHANGED)
func (app *BrowserApp) loadURL(targetURL string) {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	if strings.HasPrefix(targetURL, "file://") {
		app.loadLocalFile(targetURL)
		return
	}

	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		if !strings.Contains(targetURL, ".") {
			targetURL = "https://www.google.com/search?q=" + url.QueryEscape(targetURL)
		} else {
			targetURL = "https://" + targetURL
		}
	}

	tab.Loading = true
	tab.LoadingStart = time.Now()
	tab.StatusMessage = "Loading..."
	tab.AddressBar = targetURL

	go func() {
		// NEW: Use cached HTTP fetching
		body, _, err := app.fetchWithCache(targetURL)
		if err != nil {
			app.handleLoadError(tab, fmt.Sprintf("Failed to load: %s", err.Error()))
			return
		}

		html := string(body)
		title := extractTitle(html)
		sanitizedHTML := sanitizeAndConvertHTML(html)

		tab.PendingHTML = sanitizedHTML
		tab.PendingTitle = title
		tab.PendingURL = targetURL
		tab.HasPending = true
		tab.Loading = false
	}()
}

// Load local file (SIGNATURE UNCHANGED)
func (app *BrowserApp) loadLocalFile(fileURL string) {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	filePath := strings.TrimPrefix(fileURL, "file://")
	tab.Loading = true
	tab.LoadingStart = time.Now()
	tab.StatusMessage = "Loading local file..."
	tab.AddressBar = fileURL

	content, err := os.ReadFile(filePath)
	if err != nil {
		app.handleLoadError(tab, fmt.Sprintf("Could not read file: %s", err.Error()))
		return
	}

	html := string(content)
	title := extractTitle(html)
	if title == "Untitled" {
		title = "Local: " + filepath.Base(filePath)
	}

	sanitizedHTML := sanitizeAndConvertHTML(html)

	tab.PendingHTML = sanitizedHTML
	tab.PendingTitle = title
	tab.PendingURL = fileURL
	tab.HasPending = true
	tab.Loading = false
}

// Handle loading errors (SIGNATURE UNCHANGED)
func (app *BrowserApp) handleLoadError(tab *Tab, errorMsg string) {
	errorHTML := fmt.Sprintf(`
		<h1>Failed to Load Page</h1>
		<p><b>URL:</b> %s</p>
		<p><b>Error:</b> %s</p>
		<hr>
		<h2>Suggestions</h2>
		<ul>
			<li>Check your internet connection</li>
			<li>Verify the URL is correct</li>
			<li>Try <a href="https://example.com">example.com</a> as a test</li>
			<li>Some sites may block minimal browsers</li>
		</ul>
		<p><i>Remember: Nowser is designed for simplicity, not complexity</i></p>
	`, tab.AddressBar, errorMsg)

	tab.PendingHTML = errorHTML
	tab.PendingTitle = "Error"
	tab.PendingURL = ""
	tab.HasPending = true
	tab.Loading = false
}

// Handle file drops (SIGNATURE UNCHANGED)
func (app *BrowserApp) handleFileDrops() {
	if rl.IsFileDropped() {
		droppedFiles := rl.LoadDroppedFiles()

		for _, filename := range droppedFiles {
			ext := strings.ToLower(filepath.Ext(filename))

			if ext == ".html" || ext == ".htm" || ext == ".txt" {
				fileURL := "file://" + filename
				app.createNewTab(fileURL, true)
				app.tabs[len(app.tabs)-1].StatusMessage = fmt.Sprintf("Opened dropped file: %s", filepath.Base(filename))
			}
		}

		rl.UnloadDroppedFiles()
	}
}

// Easing function for smooth animations (SIGNATURE UNCHANGED)
func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - 4*(1-t)*(1-t)*(1-t)
}

// Toggle chrome visibility (SIGNATURE UNCHANGED)
func (app *BrowserApp) toggleChrome() {
	if app.chromeAnimating {
		return
	}

	app.chromeVisible = !app.chromeVisible
	app.chromeAnimating = true
	app.animationStart = time.Now()
}

// Update chrome animation (SIGNATURE UNCHANGED)
func (app *BrowserApp) updateChromeAnimation() {
	if !app.chromeAnimating {
		return
	}

	elapsed := time.Since(app.animationStart)
	progress := float32(elapsed.Nanoseconds()) / float32(app.animationDuration.Nanoseconds())

	if progress >= 1.0 {
		progress = 1.0
		app.chromeAnimating = false
	}

	easedProgress := easeInOutCubic(progress)
	totalChromeHeight := app.getTotalChromeHeight()

	if app.chromeVisible {
		app.chromeOffset = totalChromeHeight * (1.0 - easedProgress)
	} else {
		app.chromeOffset = totalChromeHeight * easedProgress
	}
}

// Check if modifier key is pressed (SIGNATURE UNCHANGED)
func (app *BrowserApp) isModifierPressed() bool {
	if app.isMac {
		return rl.IsKeyDown(rl.KeyLeftSuper) || rl.IsKeyDown(rl.KeyRightSuper)
	}
	return rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)
}

// Handle address bar input (SIGNATURE UNCHANGED)
func (app *BrowserApp) handleAddressBarInput() {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	for {
		char := rl.GetCharPressed()
		if char == 0 {
			break
		}

		if char >= 32 && char <= 126 {
			tab.AddressBar = tab.AddressBar[:app.cursorPos] + string(rune(char)) + tab.AddressBar[app.cursorPos:]
			app.cursorPos++
		}
	}

	if rl.IsKeyPressed(rl.KeyBackspace) && app.cursorPos > 0 {
		tab.AddressBar = tab.AddressBar[:app.cursorPos-1] + tab.AddressBar[app.cursorPos:]
		app.cursorPos--
	}

	if rl.IsKeyPressed(rl.KeyDelete) && app.cursorPos < len(tab.AddressBar) {
		tab.AddressBar = tab.AddressBar[:app.cursorPos] + tab.AddressBar[app.cursorPos+1:]
	}

	if rl.IsKeyPressed(rl.KeyLeft) && app.cursorPos > 0 {
		app.cursorPos--
	}

	if rl.IsKeyPressed(rl.KeyRight) && app.cursorPos < len(tab.AddressBar) {
		app.cursorPos++
	}

	if rl.IsKeyPressed(rl.KeyHome) {
		app.cursorPos = 0
	}

	if rl.IsKeyPressed(rl.KeyEnd) {
		app.cursorPos = len(tab.AddressBar)
	}

	if rl.IsKeyPressed(rl.KeyEnter) {
		app.editingAddress = false
		app.loadURL(tab.AddressBar)
	}

	if rl.IsKeyPressed(rl.KeyEscape) {
		app.editingAddress = false
		tab.AddressBar = tab.URL
		app.cursorPos = len(tab.AddressBar)
	}
}

// Render tab bar with dynamic sizing (SIGNATURE UNCHANGED)
func (app *BrowserApp) renderTabBar() {
	tabBarY := -app.chromeOffset
	tabWidth := app.getTabWidth()

	if tabBarY < -app.tabHeight {
		return
	}

	// Background for tab bar
	rl.DrawRectangle(0, 0, int32(app.windowWidth), int32(app.tabHeight), rl.Color{R: 240, G: 240, B: 240, A: 255})
	rl.DrawLine(0, int32(app.tabHeight), int32(app.windowWidth), int32(app.tabHeight), rl.Gray)

	currentX := float32(0)
	app.handleTabDrag()

	for i, tab := range app.tabs {
		isActive := i == app.activeTabIndex
		isDragging := app.draggingTab && i == app.dragTabIndex
		isPotentialDrag := app.potentialDragTab == i

		tabX := currentX
		if isDragging {
			tabX += app.dragOffset
		} else if app.draggingTab && i > app.dragTabIndex && i <= app.dragTargetIndex {
			tabX -= tabWidth
		} else if app.draggingTab && i < app.dragTabIndex && i >= app.dragTargetIndex {
			tabX += tabWidth
		}

		// Skip rendering tabs that are completely off-screen
		if tabX > app.windowWidth || tabX+tabWidth < 0 {
			currentX += tabWidth
			continue
		}

		tabColor := rl.Color{R: 220, G: 220, B: 220, A: 255}
		if isActive {
			tabColor = rl.White
		}
		if isDragging {
			tabColor = rl.Color{R: 200, G: 220, B: 255, A: 255}
		}
		if isPotentialDrag {
			tabColor = rl.Color{R: 235, G: 235, B: 235, A: 255}
		}

		tabRect := rl.NewRectangle(tabX, tabBarY, tabWidth, app.tabHeight)
		rl.DrawRectangleRec(tabRect, tabColor)

		if isActive {
			rl.DrawRectangleLinesEx(tabRect, 1, rl.Gray)
		} else {
			rl.DrawLine(int32(tabX+tabWidth), int32(tabBarY), int32(tabX+tabWidth), int32(app.tabHeight), rl.Gray)
		}

		// Tab title (adjust length based on tab width)
		title := tab.Title
		maxChars := int(tabWidth/8) - 3 // Rough calculation based on character width
		if maxChars < 5 {
			maxChars = 5
		}
		if len(title) > maxChars {
			title = title[:maxChars-3] + "..."
		}

		textColor := rl.DarkGray
		if isActive {
			textColor = rl.Black
		}

		if tab.Loading {
			title = "âŸ³ " + title
		}

		rl.DrawText(title, int32(tabX+8), int32(tabBarY+10), 10, textColor)

		// Close button (only if tab is wide enough and not dragging)
		if len(app.tabs) > 1 && !isDragging && tabWidth > 60 {
			closeX := tabX + tabWidth - 20
			closeY := tabBarY + 10
			closeColor := rl.Gray

			mousePos := rl.GetMousePosition()
			closeRect := rl.NewRectangle(closeX-6, closeY, 12, 12)
			if rl.CheckCollisionPointRec(mousePos, closeRect) {
				highlightRect := rl.NewRectangle(closeX-6, closeY-2, 16, 16)
				rl.DrawRectangleRec(highlightRect, rl.Color{R: 255, G: 200, B: 200, A: 150})
				rl.DrawRectangleLinesEx(highlightRect, 1, rl.Color{R: 50, G: 50, B: 50, A: 70})
				closeColor = rl.Red

				if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
					app.closeTab(i)
					app.potentialDragTab = -1
					app.draggingTab = false
					return
				}
			}

			rl.DrawText("Ã—", int32(closeX), int32(closeY), 10, closeColor)
			rl.DrawText("Ã—", int32(closeX+0.5), int32(closeY), 10, closeColor)
			rl.DrawText("Ã—", int32(closeX-0.5), int32(closeY), 10, closeColor)
		}

		currentX += tabWidth
	}

	// Draw drop indicator when dragging
	if app.draggingTab {
		dropX := float32(app.dragTargetIndex) * tabWidth
		rl.DrawRectangle(int32(dropX), int32(tabBarY), 3, int32(app.tabHeight), rl.Blue)
	}

	// New tab button (only if there's space)
	newTabX := currentX
	if newTabX+30 <= app.windowWidth {
		newTabRect := rl.NewRectangle(newTabX, tabBarY, 30, app.tabHeight)

		mousePos := rl.GetMousePosition()
		newTabColor := rl.Color{R: 220, G: 220, B: 220, A: 255}
		if rl.CheckCollisionPointRec(mousePos, newTabRect) {
			newTabColor = rl.Color{R: 200, G: 200, B: 200, A: 255}
			rl.SetMouseCursor(rl.MouseCursorPointingHand)

			if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				app.createNewTab("https://", false)
			}
		}

		rl.DrawRectangleRec(newTabRect, newTabColor)
		rl.DrawRectangleLinesEx(newTabRect, 1, rl.Gray)
		rl.DrawText("+", int32(newTabX+12), int32(tabBarY+10), 10, rl.DarkGray)
	}
}

// Render address bar with dynamic sizing (SIGNATURE UNCHANGED)
func (app *BrowserApp) renderAddressBar() {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	barY := app.tabHeight - app.chromeOffset
	barHeight := app.addressBarHeight

	if barY < -barHeight {
		return
	}

	// Background
	rl.DrawRectangle(0, int32(barY), int32(app.windowWidth), int32(barHeight), rl.Color{R: 250, G: 250, B: 250, A: 255})
	rl.DrawLine(0, int32(barY+barHeight), int32(app.windowWidth), int32(barY+barHeight), rl.Gray)

	// Address bar rectangle (dynamic width)
	padding := float32(10)
	barRect := rl.NewRectangle(padding, barY+5, app.windowWidth-2*padding, 30)
	barColor := rl.White
	if app.editingAddress {
		barColor = rl.Color{R: 255, G: 255, B: 240, A: 255}
	}

	rl.DrawRectangleRec(barRect, barColor)
	rl.DrawRectangleLinesEx(barRect, 1, rl.Gray)

	// Handle address bar click
	mousePos := rl.GetMousePosition()
	if (app.chromeVisible || app.chromeAnimating) && !app.draggingTab && app.potentialDragTab == -1 && rl.CheckCollisionPointRec(mousePos, barRect) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		app.editingAddress = true
		app.cursorPos = len(tab.AddressBar)
	}

	// Render address text (truncated if too wide for the bar)
	displayText := tab.AddressBar
	if !app.editingAddress && tab.URL != "" {
		displayText = tab.URL
	}

	// Calculate maximum characters that fit in the address bar
	maxWidth := int32(barRect.Width - 20) // Leave some padding
	textWidth := rl.MeasureText(displayText, 10)
	if textWidth > maxWidth && len(displayText) > 0 {
		// Truncate from the beginning for URLs (more useful to see the end)
		charWidth := textWidth / int32(len(displayText))
		maxChars := maxWidth / charWidth
		if maxChars > 3 {
			displayText = "..." + displayText[len(displayText)-int(maxChars)+3:]
		}
	}

	textColor := rl.Black
	if !app.editingAddress {
		textColor = rl.DarkGray
	}

	rl.DrawText(displayText, int32(padding+5), int32(barY+15), 10, textColor)

	// Draw cursor when editing
	if app.editingAddress {
		cursorText := tab.AddressBar[:app.cursorPos]
		cursorTextWidth := rl.MeasureText(cursorText, 10)

		// Adjust cursor position if text is truncated
		cursorX := int32(padding + 5 + float32(cursorTextWidth))

		if int(time.Now().UnixMilli()/500)%2 == 0 {
			rl.DrawLine(cursorX, int32(barY+12), cursorX, int32(barY+25), rl.Black)
		}
	}
}

// Render browser content with dynamic sizing (SIGNATURE UNCHANGED)
func (app *BrowserApp) renderContent() {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	x, y, width, height := app.getContentArea()

	if tab.Widget != nil {
		tab.Widget.Render(x, y, width, height)
	}
}

// Render status bar with dynamic sizing and cache info (ENHANCED)
func (app *BrowserApp) renderStatusBar() {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	statusY := app.windowHeight - app.statusBarHeight + app.chromeOffset

	if statusY > app.windowHeight {
		return
	}

	// Background
	rl.DrawRectangle(0, int32(statusY), int32(app.windowWidth), int32(app.statusBarHeight), rl.Color{R: 240, G: 240, B: 240, A: 255})
	rl.DrawLine(0, int32(statusY), int32(app.windowWidth), int32(statusY), rl.Gray)

	// Status message (enhanced with cache info)
	message := tab.StatusMessage
	if tab.Loading {
		elapsed := time.Since(tab.LoadingStart)
		message = fmt.Sprintf("Loading... (%dms)", elapsed.Milliseconds())
	} else if strings.Contains(tab.StatusMessage, "Loaded in") {
		// Check if current page was served from cache
		if entry, found := app.cacheManager.Get(tab.URL, ResourceTypePage); found {
			cacheAge := time.Since(entry.CachedAt)
			if cacheAge < time.Hour {
				message = fmt.Sprintf("%s (cached %s ago)", tab.StatusMessage,
					formatDuration(cacheAge))
			}
		}
	}

	rl.DrawText(message, 10, int32(statusY+8), 10, rl.DarkGray)

	// Keyboard shortcuts (adjust based on window width)
	hints := fmt.Sprintf("%s+T: New | %s+W: Close | %s+L: Address | Middle Click: Toggle Chrome",
		app.modifierKey, app.modifierKey, app.modifierKey)

	// Shorten hints if window is narrow
	if app.windowWidth < 800 {
		hints = fmt.Sprintf("%s+T | %s+W | %s+L | Mid Click", app.modifierKey, app.modifierKey, app.modifierKey)
	}
	if app.windowWidth < 600 {
		hints = "Mid Click: Toggle UI"
	}

	hintsWidth := rl.MeasureText(hints, 10)
	hintsX := int32(app.windowWidth - float32(hintsWidth) - 10)
	if hintsX > 10 { // Only draw if there's space
		rl.DrawText(hints, hintsX, int32(statusY+10), 10, rl.Gray)
	}
}

// NEW: Helper function to format durations nicely
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

// Handle keyboard shortcuts (SIGNATURE UNCHANGED)
func (app *BrowserApp) handleKeyboard() {
	if app.isModifierPressed() {
		if rl.IsKeyPressed(rl.KeyT) {
			if !app.chromeVisible && !app.chromeAnimating {
				app.toggleChrome()
			}
			app.createNewTab("https://", false)
		}
		if rl.IsKeyPressed(rl.KeyW) {
			if len(app.tabs) > 1 {
				app.closeTab(app.activeTabIndex)
			}
		}
		if rl.IsKeyPressed(rl.KeyL) {
			if !app.chromeVisible && !app.chromeAnimating {
				app.toggleChrome()
			}
			app.editingAddress = true
			tab := app.currentTab()
			if tab != nil {
				app.cursorPos = len(tab.AddressBar)
			}
		}
		if rl.IsKeyPressed(rl.KeyR) {
			tab := app.currentTab()
			if tab != nil && tab.URL != "" {
				app.loadURL(tab.URL)
			}
		}
		// NEW: Cache management shortcuts
		if rl.IsKeyPressed(rl.KeyK) {
			// Ctrl+K to clear cache
			app.cacheManager.caches[ResourceTypePage].Clear()
			if tab := app.currentTab(); tab != nil {
				tab.StatusMessage = "Cache cleared"
			}
		}
	}

	// Tab switching with number keys
	if app.isModifierPressed() {
		for i := rl.KeyOne; i <= rl.KeyNine; i++ {
			if rl.IsKeyPressed(int32(i)) {
				tabIndex := int(i - rl.KeyOne)
				if tabIndex < len(app.tabs) {
					app.activeTabIndex = tabIndex
					app.editingAddress = false
				}
			}
		}
	}

	if rl.IsKeyPressed(rl.KeyEscape) {
		if app.editingAddress {
			app.editingAddress = false
		} else {
			os.Exit(0)
		}
	}

	if app.editingAddress && (app.chromeVisible || app.chromeAnimating) {
		app.handleAddressBarInput()
	}
}

// Update application state (SIGNATURE UNCHANGED)
func (app *BrowserApp) update() {
	// Update window dimensions first
	app.updateWindowDimensions()

	rl.SetMouseCursor(rl.MouseCursorDefault)

	if rl.IsMouseButtonPressed(rl.MouseButtonMiddle) {
		app.toggleChrome()
		if !app.chromeVisible {
			app.editingAddress = false
		}
	}

	app.updateChromeAnimation()
	app.handleFileDrops()

	// Process pending content updates
	for _, tab := range app.tabs {
		if tab.HasPending {
			if tab.Widget != nil {
				tab.Widget.Unload()
			}
			tab.Widget = marquee.NewHTMLWidget(tab.PendingHTML)
			tab.setupLinkHandler(app)

			tab.URL = tab.PendingURL
			tab.Title = tab.PendingTitle

			if tab.PendingURL != "" && (len(app.history) == 0 || app.history[len(app.history)-1] != tab.PendingURL) {
				app.history = append(app.history, tab.PendingURL)
				app.historyIndex = len(app.history) - 1
			}

			elapsed := time.Since(tab.LoadingStart)
			tab.StatusMessage = fmt.Sprintf("Loaded in %dms", elapsed.Milliseconds())
			tab.HasPending = false
			app.saveSession()
		}
	}

	app.handleKeyboard()

	// Update current tab widget
	tab := app.currentTab()
	if tab != nil && tab.Widget != nil && !app.editingAddress {
		tab.Widget.Update()
	}

	// Periodically save cache and scroll positions
	if int(time.Now().Unix())%10 == 0 {
		app.cacheManager.SaveToDisk()
		app.saveSession()
	}

	// Click outside address bar to stop editing
	if app.editingAddress && (app.chromeVisible || app.chromeAnimating) {
		mousePos := rl.GetMousePosition()
		padding := float32(10)
		addressRect := rl.NewRectangle(padding, app.tabHeight+5-app.chromeOffset, app.windowWidth-2*padding, 30)
		if !rl.CheckCollisionPointRec(mousePos, addressRect) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			app.editingAddress = false
		}
	}
}

// Cleanup resources (ENHANCED)
func (app *BrowserApp) cleanup() {
	// Save session and cache
	app.saveSession()
	app.cacheManager.SaveToDisk()

	for _, tab := range app.tabs {
		if tab.Widget != nil {
			tab.Widget.Unload()
		}
	}
}

func main() {
	rl.SetConfigFlags(rl.FlagWindowResizable)

	rl.InitWindow(900, 700, "Nowser")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)

	app := NewBrowserApp()
	defer app.cleanup()

	// Load URL from command line if provided
	if len(os.Args) > 1 {
		app.loadURL(os.Args[1])
	}

	// Main loop
	for !rl.WindowShouldClose() {
		app.update()

		rl.BeginDrawing()
		rl.ClearBackground(rl.Color{R: 245, G: 245, B: 245, A: 255})

		app.renderContent()

		if app.chromeVisible || app.chromeAnimating {
			app.renderTabBar()
			app.renderAddressBar()
			app.renderStatusBar()
		}

		// Show adaptive hint when chrome is hidden
		if !app.chromeVisible && !app.chromeAnimating {
			hintAlpha := uint8(80)
			hintColor := rl.Color{R: 100, G: 100, B: 100, A: hintAlpha}
			hintText := "Middle click to show controls"

			// Position hint based on window size
			hintWidth := rl.MeasureText(hintText, 10)
			hintX := int32(app.windowWidth - float32(hintWidth) - 10)
			if hintX < 10 {
				hintX = 10 // Prevent hint from going off-screen
			}

			rl.DrawText(hintText, hintX, 10, 10, hintColor)
		}

		rl.EndDrawing()
	}
}
