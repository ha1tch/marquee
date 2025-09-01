///////////////////////////////////////
// The Nowser Antiexplorer           //
// Powered by MARQUEE and Bloatscape //
// --------------------------------- //
// Version 0.4 - Tab Click & Drop    //
///////////////////////////////////////

package main

import (
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
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/marquee"
)

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
	isMac            bool
	modifierKey      string // "Cmd" or "Ctrl"
	// Chrome animation
	chromeVisible     bool
	chromeAnimating   bool
	animationStart    time.Time
	animationDuration time.Duration
	chromeOffset      float32 // Current offset for animation
	// Tab drag & drop - improved to prevent accidental dragging
	draggingTab      bool
	dragTabIndex     int
	dragStartX       float32
	dragStartY       float32
	dragOffset       float32
	dragTargetIndex  int
	potentialDragTab int // Tab that might be dragged
	dragStartTime    time.Time
	dragThreshold    float32 // Minimum distance to start drag
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
		isMac:             isMac,
		modifierKey:       modifierKey,
		chromeVisible:     true,
		chromeAnimating:   false,
		animationDuration: 350 * time.Millisecond, // Smooth 350ms animation
		chromeOffset:      0,
		draggingTab:       false,
		dragTabIndex:      -1,
		dragTargetIndex:   -1,
		potentialDragTab:  -1,
		dragThreshold:     8.0, // Minimum 8 pixels to start drag
	}

	// Try to load saved session
	if !app.loadSession() {
		// No saved session, create initial tab
		// Check if index.html exists in current directory
		startURL := "https://example.com"
		if _, err := os.Stat("index.html"); err == nil {
			// Convert index.html to file:// URL
			pwd, _ := os.Getwd()
			startURL = "file://" + pwd + "/index.html"
		}

		app.createNewTab(startURL, true)
	}

	return app
}

// Get .nowser directory path
func getNowserDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".nowser" // Fallback to current directory
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
		if tab.URL != "" { // Only save tabs with actual URLs
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

	// Don't save if no valid tabs
	if len(session.Tabs) == 0 {
		return
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return // Silently fail
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
		return false // No session file
	}

	var session BrowserSession
	if err := json.Unmarshal(data, &session); err != nil {
		return false // Invalid session file
	}

	if len(session.Tabs) == 0 {
		return false // Empty session
	}

	// Restore tabs
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

	// Restore active tab index
	if session.ActiveTabIndex >= 0 && session.ActiveTabIndex < len(app.tabs) {
		app.activeTabIndex = session.ActiveTabIndex
	}

	// Load content for active tab first
	if app.activeTabIndex < len(app.tabs) {
		app.loadURL(app.tabs[app.activeTabIndex].URL)
	}

	return true
}

// Handle tab drag and drop - improved to prevent accidental dragging
func (app *BrowserApp) handleTabDrag() {
	mousePos := rl.GetMousePosition()

	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && !app.draggingTab && app.potentialDragTab == -1 {
		// Check if starting potential drag on a tab
		tabWidth := float32(200)
		maxTabWidth := float32(900) / float32(len(app.tabs))
		if maxTabWidth < tabWidth {
			tabWidth = maxTabWidth
		}

		tabBarY := -app.chromeOffset
		currentX := float32(0)

		for i := range app.tabs {
			tabRect := rl.NewRectangle(currentX, tabBarY, tabWidth, app.tabHeight)

			if rl.CheckCollisionPointRec(mousePos, tabRect) {
				// Check if not clicking close button
				closeX := currentX + tabWidth - 20
				if mousePos.X < closeX {
					// Start potential drag - don't immediately enter drag mode
					app.potentialDragTab = i
					app.dragStartX = mousePos.X
					app.dragStartY = mousePos.Y
					app.dragStartTime = time.Now()
					return // Don't process tab click yet
				}
			}
			currentX += tabWidth
		}
	}

	// Handle potential drag state
	if app.potentialDragTab != -1 {
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			// Check if mouse has moved enough to start dragging
			distance := float32(0)
			if mousePos.X > app.dragStartX {
				distance = mousePos.X - app.dragStartX
			} else {
				distance = app.dragStartX - mousePos.X
			}

			// Also check Y distance
			yDistance := float32(0)
			if mousePos.Y > app.dragStartY {
				yDistance = mousePos.Y - app.dragStartY
			} else {
				yDistance = app.dragStartY - mousePos.Y
			}

			totalDistance := distance + yDistance

			if totalDistance > app.dragThreshold {
				// Start actual dragging
				app.draggingTab = true
				app.dragTabIndex = app.potentialDragTab
				app.dragOffset = 0
				app.dragTargetIndex = app.potentialDragTab
				app.potentialDragTab = -1
			}
		} else {
			// Mouse released without dragging - this is a tab click
			clickedTab := app.tabs[app.potentialDragTab]
			app.activeTabIndex = app.potentialDragTab
			app.editingAddress = false

			if clickedTab.Widget == nil && clickedTab.URL != "" {
				app.loadURL(clickedTab.URL)
			}

			app.potentialDragTab = -1
		}
	}

	// Handle active dragging
	if app.draggingTab {
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			// Update drag offset
			app.dragOffset = mousePos.X - app.dragStartX

			// Calculate target drop position
			tabWidth := float32(200)
			maxTabWidth := float32(900) / float32(len(app.tabs))
			if maxTabWidth < tabWidth {
				tabWidth = maxTabWidth
			}

			//draggedTabCenter := app.dragStartX + app.dragOffset
			//app.dragTargetIndex = int(draggedTabCenter / tabWidth)
			app.dragTargetIndex = int(mousePos.X / tabWidth)

			// Clamp to valid range
			if app.dragTargetIndex < 0 {
				app.dragTargetIndex = 0
			}
			if app.dragTargetIndex >= len(app.tabs) {
				app.dragTargetIndex = len(app.tabs) - 1
			}
		} else {
			// Mouse released - complete the drag
			if app.dragTargetIndex != app.dragTabIndex {
				app.reorderTabs(app.dragTabIndex, app.dragTargetIndex)
			}

			app.draggingTab = false
			app.dragTabIndex = -1
			app.dragTargetIndex = -1
			app.dragOffset = 0

			// Save session after reordering
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

	// Remove tab from old position
	tab := app.tabs[oldIndex]
	app.tabs = append(app.tabs[:oldIndex], app.tabs[oldIndex+1:]...)

	//if newIndex > oldIndex {
	//	newIndex-- // Adjust for removed element
	//}

	app.tabs = append(app.tabs[:newIndex], append([]*Tab{tab}, app.tabs[newIndex:]...)...)

	// Update active tab index
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

	// Create welcome content for new tabs
	if !loadImmediately {
		welcomeHTML := fmt.Sprintf(`
			<h1>Nowser</h1>
			<p>A lightweight, <i>minimalist</i> wowser built with <b>MARQUEE</b></p>
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
			</ul>
			<h3>Philosophy</h3>
			<p>In an age of <i>bloated browsers</i>, Nowser embraces <b>unbloaticity</b>. No extensions, no trackers, no complexity - just pure browsing.</p>
			<p>Hide the browser chrome for <i>distraction-free reading</i> and focus on content.</p>
			<p>Try visiting: <a href="https://example.com">example.com</a> or <a href="https://www.berkshirehathaway.com">berkshirehathaway.com</a></p>
		`, app.modifierKey, app.modifierKey, app.modifierKey)
		tab.Widget = marquee.NewHTMLWidget(welcomeHTML)
		tab.Title = "Welcome to Nowser"
		tab.setupLinkHandler(app)
	}

	app.tabs = append(app.tabs, tab)
	app.activeTabIndex = len(app.tabs) - 1

	if loadImmediately {
		app.loadURL(initialURL)
	}

	// Save session when creating new tabs
	app.saveSession()
}

// Setup link click handler for a tab
func (tab *Tab) setupLinkHandler(app *BrowserApp) {
	if tab.Widget == nil {
		return
	}

	tab.Widget.OnLinkClick = func(clickedURL string) {
		// Handle relative URLs
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
		return // Don't close the last tab
	}

	// Clean up widget
	if app.tabs[index].Widget != nil {
		app.tabs[index].Widget.Unload()
	}

	// Remove tab from slice
	app.tabs = append(app.tabs[:index], app.tabs[index+1:]...)

	// Adjust active tab index
	if app.activeTabIndex >= len(app.tabs) {
		app.activeTabIndex = len(app.tabs) - 1
	} else if app.activeTabIndex > index {
		app.activeTabIndex--
	}

	// Save session after closing tab
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
	// Remove script and style tags
	scriptRe := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")
	styleRe := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Remove unsupported tags but keep content
	unsupportedTags := []string{"div", "span", "section", "article", "nav", "header", "footer", "main", "aside", "meta", "link"}
	for _, tag := range unsupportedTags {
		openRe := regexp.MustCompile(fmt.Sprintf(`(?i)<%s[^>]*>`, tag))
		closeRe := regexp.MustCompile(fmt.Sprintf(`(?i)</%s>`, tag))
		html = openRe.ReplaceAllString(html, "")
		html = closeRe.ReplaceAllString(html, "")
	}

	// Convert HTML entities
	entities := map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   "\"",
		"&nbsp;":   " ",
		"&mdash;":  "–",
		"&ndash;":  "–",
		"&hellip;": "…",
	}

	for entity, replacement := range entities {
		html = strings.ReplaceAll(html, entity, replacement)
	}

	// Extract body content if present, otherwise use entire content
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

// Load URL in current tab
func (app *BrowserApp) loadURL(targetURL string) {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	// Handle file:// URLs
	if strings.HasPrefix(targetURL, "file://") {
		app.loadLocalFile(targetURL)
		return
	}

	// Validate and normalize URL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		if !strings.Contains(targetURL, ".") {
			// Treat as search query
			targetURL = "https://www.google.com/search?q=" + url.QueryEscape(targetURL)
		} else {
			targetURL = "https://" + targetURL
		}
	}

	tab.Loading = true
	tab.LoadingStart = time.Now()
	tab.StatusMessage = "Loading..."
	tab.AddressBar = targetURL

	// Fetch content in goroutine to avoid blocking UI
	go func() {
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		resp, err := client.Get(targetURL)
		if err != nil {
			app.handleLoadError(tab, fmt.Sprintf("Failed to connect: %s", err.Error()))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			app.handleLoadError(tab, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status))
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			app.handleLoadError(tab, fmt.Sprintf("Failed to read response: %s", err.Error()))
			return
		}

		html := string(body)

		// Extract title and sanitize content
		title := extractTitle(html)
		sanitizedHTML := sanitizeAndConvertHTML(html)

		// Set pending content for main thread processing
		tab.PendingHTML = sanitizedHTML
		tab.PendingTitle = title
		tab.PendingURL = targetURL
		tab.HasPending = true
		tab.Loading = false
	}()
}

// Load local file
func (app *BrowserApp) loadLocalFile(fileURL string) {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	// Extract file path from file:// URL
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

	// Set pending content for main thread processing
	tab.PendingHTML = sanitizedHTML
	tab.PendingTitle = title
	tab.PendingURL = fileURL
	tab.HasPending = true
	tab.Loading = false
}

// Handle loading errors
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

	// Set pending error content for main thread processing
	tab.PendingHTML = errorHTML
	tab.PendingTitle = "Error"
	tab.PendingURL = ""
	tab.HasPending = true
	tab.Loading = false
}

// Handle file drops - new feature
func (app *BrowserApp) handleFileDrops() {
	if rl.IsFileDropped() {
		droppedFiles := rl.LoadDroppedFiles()

		for _, filename := range droppedFiles {
			ext := strings.ToLower(filepath.Ext(filename))

			// Check if it's a supported file type
			if ext == ".html" || ext == ".htm" || ext == ".txt" {
				// Convert to file:// URL
				fileURL := "file://" + filename

				// Create new tab for the dropped file
				app.createNewTab(fileURL, true)

				// Update status
				app.tabs[len(app.tabs)-1].StatusMessage = fmt.Sprintf("Opened dropped file: %s", filepath.Base(filename))
			}
		}

		rl.UnloadDroppedFiles()
	}
}

// Easing function for smooth animations (ease-in-out cubic)
func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - 4*(1-t)*(1-t)*(1-t)
}

// Toggle chrome visibility
func (app *BrowserApp) toggleChrome() {
	if app.chromeAnimating {
		return // Don't interrupt ongoing animation
	}

	app.chromeVisible = !app.chromeVisible
	app.chromeAnimating = true
	app.animationStart = time.Now()
}

// Update chrome animation
func (app *BrowserApp) updateChromeAnimation() {
	if !app.chromeAnimating {
		return
	}

	elapsed := time.Since(app.animationStart)
	progress := float32(elapsed.Nanoseconds()) / float32(app.animationDuration.Nanoseconds())

	if progress >= 1.0 {
		// Animation complete
		progress = 1.0
		app.chromeAnimating = false
	}

	// Apply easing
	easedProgress := easeInOutCubic(progress)

	// Calculate offset
	totalChromeHeight := app.tabHeight + app.addressBarHeight + 30 // +30 for status bar

	if app.chromeVisible {
		// Animating from hidden to visible
		app.chromeOffset = totalChromeHeight * (1.0 - easedProgress)
	} else {
		// Animating from visible to hidden
		app.chromeOffset = totalChromeHeight * easedProgress
	}
}

// Check if modifier key is pressed (Cmd on Mac, Ctrl elsewhere)
func (app *BrowserApp) isModifierPressed() bool {
	if app.isMac {
		return rl.IsKeyDown(rl.KeyLeftSuper) || rl.IsKeyDown(rl.KeyRightSuper)
	}
	return rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)
}

// Handle address bar input
func (app *BrowserApp) handleAddressBarInput() {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	// Get typed characters
	for {
		char := rl.GetCharPressed()
		if char == 0 {
			break
		}

		// Insert character at cursor position
		if char >= 32 && char <= 126 { // Printable ASCII
			tab.AddressBar = tab.AddressBar[:app.cursorPos] + string(rune(char)) + tab.AddressBar[app.cursorPos:]
			app.cursorPos++
		}
	}

	// Handle special keys
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

	// Load URL on Enter
	if rl.IsKeyPressed(rl.KeyEnter) {
		app.editingAddress = false
		app.loadURL(tab.AddressBar)
	}

	// Cancel editing on Escape
	if rl.IsKeyPressed(rl.KeyEscape) {
		app.editingAddress = false
		tab.AddressBar = tab.URL
		app.cursorPos = len(tab.AddressBar)
	}
}

// Render tab bar
func (app *BrowserApp) renderTabBar() {
	tabBarY := -app.chromeOffset // Apply animation offset
	tabWidth := float32(200)
	maxTabWidth := float32(900) / float32(len(app.tabs))
	if maxTabWidth < tabWidth {
		tabWidth = maxTabWidth
	}

	// Skip rendering if completely off-screen
	if tabBarY < -app.tabHeight {
		return
	}

	// Background for tab bar
	rl.DrawRectangle(0, 0, 900, int32(app.tabHeight), rl.Color{R: 240, G: 240, B: 240, A: 255})
	rl.DrawLine(0, int32(app.tabHeight), 900, int32(app.tabHeight), rl.Gray)

	currentX := float32(0)

	// Handle tab dragging
	app.handleTabDrag()

	for i, tab := range app.tabs {
		isActive := i == app.activeTabIndex
		isDragging := app.draggingTab && i == app.dragTabIndex
		isPotentialDrag := app.potentialDragTab == i

		// Calculate tab position (with drag offset if dragging)
		tabX := currentX
		if isDragging {
			tabX += app.dragOffset
		} else if app.draggingTab && i > app.dragTabIndex && i <= app.dragTargetIndex {
			// Shift left when dragging right
			tabX -= tabWidth
		} else if app.draggingTab && i < app.dragTabIndex && i >= app.dragTargetIndex {
			// Shift right when dragging left
			tabX += tabWidth
		}

		// Tab background
		tabColor := rl.Color{R: 220, G: 220, B: 220, A: 255}
		if isActive {
			tabColor = rl.White
		}
		if isDragging {
			tabColor = rl.Color{R: 200, G: 220, B: 255, A: 255} // Slight blue tint when dragging
		}
		if isPotentialDrag {
			tabColor = rl.Color{R: 235, G: 235, B: 235, A: 255} // Slight highlight for potential drag
		}

		tabRect := rl.NewRectangle(tabX, tabBarY, tabWidth, app.tabHeight)
		rl.DrawRectangleRec(tabRect, tabColor)

		// Tab border
		if isActive {
			rl.DrawRectangleLinesEx(tabRect, 1, rl.Gray)
		} else {
			rl.DrawLine(int32(tabX+tabWidth), int32(tabBarY), int32(tabX+tabWidth), int32(app.tabHeight), rl.Gray)
		}

		// Tab title (truncated if needed)
		title := tab.Title
		if len(title) > 20 {
			title = title[:17] + "..."
		}

		textColor := rl.DarkGray
		if isActive {
			textColor = rl.Black
		}

		// Loading indicator
		if tab.Loading {
			title = "⟳ " + title
		}

		rl.DrawText(title, int32(tabX+8), int32(tabBarY+10), 10, textColor)

		// Close button (X) - only if not dragging
		if len(app.tabs) > 1 && !isDragging {
			closeX := tabX + tabWidth - 20
			closeY := tabBarY + 10
			closeColor := rl.Gray

			// Check if mouse is over close button
			mousePos := rl.GetMousePosition()
			closeRect := rl.NewRectangle(closeX-6, closeY, 12, 12)
			if rl.CheckCollisionPointRec(mousePos, closeRect) {
				// Draw background highlight instead of changing cursor
				highlightRect := rl.NewRectangle(closeX-6, closeY-2, 16, 16)

				rl.DrawRectangleRec(highlightRect, rl.Color{R: 255, G: 200, B: 200, A: 150})    // Light red background
				rl.DrawRectangleLinesEx(highlightRect, 1, rl.Color{R: 50, G: 50, B: 50, A: 70}) // Stroke

				closeColor = rl.Red // Darker red text

				// Handle close click
				if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
					app.closeTab(i)
					// Reset drag state when closing
					app.potentialDragTab = -1
					app.draggingTab = false
					return
				}
			}

			rl.DrawText("×", int32(closeX), int32(closeY), 10, closeColor)
			rl.DrawText("×", int32(closeX+0.5), int32(closeY), 10, closeColor)
			rl.DrawText("×", int32(closeX-0.5), int32(closeY), 10, closeColor)
		}

		currentX += tabWidth
	}

	// Draw drop indicator when dragging
	if app.draggingTab {
		dropX := float32(app.dragTargetIndex) * tabWidth
		rl.DrawRectangle(int32(dropX), int32(tabBarY), 3, int32(app.tabHeight), rl.Blue)
	}

	// New tab button
	newTabX := currentX
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

// Render address bar
func (app *BrowserApp) renderAddressBar() {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	barY := app.tabHeight - app.chromeOffset // Apply animation offset
	barHeight := app.addressBarHeight

	// Skip rendering if completely off-screen
	if barY < -barHeight {
		return
	}

	// Background
	rl.DrawRectangle(0, int32(barY), 900, int32(barHeight), rl.Color{R: 250, G: 250, B: 250, A: 255})
	rl.DrawLine(0, int32(barY+barHeight), 900, int32(barY+barHeight), rl.Gray)

	// Address bar rectangle
	barRect := rl.NewRectangle(10, barY+5, 880, 30)
	barColor := rl.White
	if app.editingAddress {
		barColor = rl.Color{R: 255, G: 255, B: 240, A: 255} // Slight yellow tint when editing
	}

	rl.DrawRectangleRec(barRect, barColor)
	rl.DrawRectangleLinesEx(barRect, 1, rl.Gray)

	// Handle address bar click (only if chrome is visible and not dragging)
	mousePos := rl.GetMousePosition()
	if (app.chromeVisible || app.chromeAnimating) && !app.draggingTab && app.potentialDragTab == -1 && rl.CheckCollisionPointRec(mousePos, barRect) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		app.editingAddress = true
		app.cursorPos = len(tab.AddressBar)
	}

	// Render address text
	displayText := tab.AddressBar
	if !app.editingAddress && tab.URL != "" {
		displayText = tab.URL
	}

	textColor := rl.Black
	if !app.editingAddress {
		textColor = rl.DarkGray
	}

	rl.DrawText(displayText, 15, int32(barY+15), 10, textColor)

	// Draw cursor when editing
	if app.editingAddress {
		cursorText := displayText[:app.cursorPos]
		cursorX := 15 + rl.MeasureText(cursorText, 10)

		// Blinking cursor
		if int(time.Now().UnixMilli()/500)%2 == 0 {
			rl.DrawLine(cursorX, int32(barY+12), cursorX, int32(barY+25), rl.Black)
		}
	}
}

// Render browser content
func (app *BrowserApp) renderContent() {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	contentY := app.tabHeight + app.addressBarHeight - app.chromeOffset // Apply animation offset
	contentHeight := 700 - contentY - 30                                // Leave space for status bar

	// Adjust for status bar animation
	if !app.chromeVisible || app.chromeAnimating {
		contentHeight = 700 - contentY + app.chromeOffset // Expand content when chrome hidden
	}

	if tab.Widget != nil {
		tab.Widget.Render(20, contentY+10, 860, contentHeight-20)
	}
}

// Render status bar
func (app *BrowserApp) renderStatusBar() {
	tab := app.currentTab()
	if tab == nil {
		return
	}

	statusY := 670 + app.chromeOffset // Apply animation offset (move down when hiding)

	// Skip rendering if completely off-screen
	if statusY > 700 {
		return
	}

	// Background
	rl.DrawRectangle(0, int32(statusY), 900, 30, rl.Color{R: 240, G: 240, B: 240, A: 255})
	rl.DrawLine(0, int32(statusY), 900, int32(statusY), rl.Gray)

	// Status message
	message := tab.StatusMessage
	if tab.Loading {
		elapsed := time.Since(tab.LoadingStart)
		message = fmt.Sprintf("Loading... (%dms)", elapsed.Milliseconds())
	}

	rl.DrawText(message, 10, int32(statusY+8), 10, rl.DarkGray)

	// Keyboard shortcuts (platform-specific)
	hints := fmt.Sprintf("%s+T: New | %s+W: Close | %s+L: Address | Middle Click: Toggle Chrome",
		app.modifierKey, app.modifierKey, app.modifierKey)
	hintsWidth := rl.MeasureText(hints, 10)
	rl.DrawText(hints, 900-hintsWidth-10, int32(statusY+10), 10, rl.Gray)
}

// Handle keyboard shortcuts
func (app *BrowserApp) handleKeyboard() {
	// Global shortcuts with platform-appropriate modifier
	if app.isModifierPressed() {
		if rl.IsKeyPressed(rl.KeyT) {
			// New tab - show chrome if hidden
			if !app.chromeVisible && !app.chromeAnimating {
				app.toggleChrome()
			}
			app.createNewTab("https://", false)
		}
		if rl.IsKeyPressed(rl.KeyW) {
			// Close tab
			if len(app.tabs) > 1 {
				app.closeTab(app.activeTabIndex)
			}
		}
		if rl.IsKeyPressed(rl.KeyL) {
			// Focus address bar - show chrome if hidden
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
			// Refresh
			tab := app.currentTab()
			if tab != nil && tab.URL != "" {
				app.loadURL(tab.URL)
			}
		}
	}

	// Tab switching with Cmd/Ctrl+1-9
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

	// Quit on Escape (when not editing address)
	if rl.IsKeyPressed(rl.KeyEscape) {
		if app.editingAddress {
			app.editingAddress = false
		} else {
			os.Exit(0)
		}
	}

	// Handle address bar input (only if chrome is visible)
	if app.editingAddress && (app.chromeVisible || app.chromeAnimating) {
		app.handleAddressBarInput()
	}
}

// Update application state
func (app *BrowserApp) update() {
	// Reset mouse cursor
	rl.SetMouseCursor(rl.MouseCursorDefault)

	// Handle middle mouse button for chrome toggle
	if rl.IsMouseButtonPressed(rl.MouseButtonMiddle) {
		app.toggleChrome()
		// Stop editing address if hiding chrome
		if !app.chromeVisible {
			app.editingAddress = false
		}
	}

	// Update chrome animation
	app.updateChromeAnimation()

	// Handle file drops - new feature
	app.handleFileDrops()

	// Process any pending content updates on main thread
	for _, tab := range app.tabs {
		if tab.HasPending {
			// Create new widget on main thread (thread-safe)
			if tab.Widget != nil {
				tab.Widget.Unload()
			}
			tab.Widget = marquee.NewHTMLWidget(tab.PendingHTML)
			tab.setupLinkHandler(app)

			// Update tab properties
			tab.URL = tab.PendingURL
			tab.Title = tab.PendingTitle

			// Update history
			if tab.PendingURL != "" && (len(app.history) == 0 || app.history[len(app.history)-1] != tab.PendingURL) {
				app.history = append(app.history, tab.PendingURL)
				app.historyIndex = len(app.history) - 1
			}

			elapsed := time.Since(tab.LoadingStart)
			tab.StatusMessage = fmt.Sprintf("Loaded in %dms", elapsed.Milliseconds())

			// Clear pending flag
			tab.HasPending = false

			// Save session when content loads
			app.saveSession()
		}
	}

	// Handle keyboard input
	app.handleKeyboard()

	// Update current tab widget
	tab := app.currentTab()
	if tab != nil && tab.Widget != nil && !app.editingAddress {
		tab.Widget.Update()
	}

	// Periodically save scroll positions (every 5 seconds)
	if int(time.Now().Unix())%5 == 0 {
		app.saveSession()
	}

	// Click outside address bar to stop editing (only if chrome is visible)
	if app.editingAddress && (app.chromeVisible || app.chromeAnimating) {
		mousePos := rl.GetMousePosition()
		addressRect := rl.NewRectangle(10, app.tabHeight+5-app.chromeOffset, 880, 30)
		if !rl.CheckCollisionPointRec(mousePos, addressRect) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			app.editingAddress = false
		}
	}
}

// Cleanup resources
func (app *BrowserApp) cleanup() {
	// Save session before cleanup
	app.saveSession()

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

	// Initialize browser
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

		// Always render content first
		app.renderContent()

		// Render chrome elements (with animation offsets)
		if app.chromeVisible || app.chromeAnimating {
			app.renderTabBar()
			app.renderAddressBar()
			app.renderStatusBar()
		}

		// Show subtle hint when chrome is hidden
		if !app.chromeVisible && !app.chromeAnimating {
			hintAlpha := uint8(80)
			hintColor := rl.Color{R: 100, G: 100, B: 100, A: hintAlpha}
			hintText := "Middle click to show controls"
			hintWidth := rl.MeasureText(hintText, 10)
			rl.DrawText(hintText, 900-hintWidth-10, 10, 10, hintColor)
		}

		rl.EndDrawing()
	}
}
