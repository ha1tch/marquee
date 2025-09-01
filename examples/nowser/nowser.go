///////////////////////////////////////
// The Nowser Antiexplorer           //
// Powered by MARQUEE and Bloatscape //
// --------------------------------- //
// Version 0.5 - Resizable Window    //
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
		welcomeHTML := fmt.Sprintf(`
			<h1>Nowser</h1>
			<p>A lightweight, <i>minimalist</i> browser built with <b>MARQUEE</b></p>
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
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&hellip;": "…",
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

// Load URL in current tab
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
		title := extractTitle(html)
		sanitizedHTML := sanitizeAndConvertHTML(html)

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

	tab.PendingHTML = errorHTML
	tab.PendingTitle = "Error"
	tab.PendingURL = ""
	tab.HasPending = true
	tab.Loading = false
}

// Handle file drops
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

// Easing function for smooth animations
func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - 4*(1-t)*(1-t)*(1-t)
}

// Toggle chrome visibility
func (app *BrowserApp) toggleChrome() {
	if app.chromeAnimating {
		return
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

// Check if modifier key is pressed
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

// Render tab bar with dynamic sizing
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
		maxChars := int(tabWidth / 8) - 3 // Rough calculation based on character width
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
			title = "⟳ " + title
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

	// New tab button (only if there's space)
	newTabX := currentX
	if newTabX + 30 <= app.windowWidth {
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

// Render address bar with dynamic sizing
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

// Render browser content with dynamic sizing
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

// Render status bar with dynamic sizing
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

	// Status message
	message := tab.StatusMessage
	if tab.Loading {
		elapsed := time.Since(tab.LoadingStart)
		message = fmt.Sprintf("Loading... (%dms)", elapsed.Milliseconds())
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

// Handle keyboard shortcuts
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

// Update application state
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

	// Periodically save scroll positions
	if int(time.Now().Unix())%5 == 0 {
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

// Cleanup resources
func (app *BrowserApp) cleanup() {
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