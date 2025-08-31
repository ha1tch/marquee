///////////////////////////////////////
// The Nowser Antiexplorer           //
// Powered by MARQUEE and Bloatscape //
// --------------------------------- //
///////////////////////////////////////

package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/marquee"
)

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
}

// NewBrowserApp creates a new browser application with one initial tab
func NewBrowserApp() *BrowserApp {
	isMac := runtime.GOOS == "darwin"
	modifierKey := "Ctrl"
	if isMac {
		modifierKey = "Cmd"
	}

	app := &BrowserApp{
		tabs:             make([]*Tab, 0),
		activeTabIndex:   0,
		tabHeight:        35,
		addressBarHeight: 40,
		isMac:            isMac,
		modifierKey:      modifierKey,
	}

	// Check if index.html exists in current directory
	startURL := "https://example.com"
	if _, err := os.Stat("index.html"); err == nil {
		// Convert index.html to file:// URL
		pwd, _ := os.Getwd()
		startURL = "file://" + pwd + "/index.html"
	}

	// Create initial tab
	app.createNewTab(startURL, true)
	return app
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
			<h1>The Nowser Antiexplorer</h1>
			<p>A lightweight, <i>minimalist</i> web browser built with <b>MARQUEE</b></p>
			<hr>
			<h2>Getting Started</h2>
			<ul>
				<li>Enter a URL in the address bar and press <b>Enter</b></li>
				<li>Use <b>%s+T</b> to open a new tab</li>
				<li>Use <b>%s+W</b> to close the current tab</li>
				<li>Use <b>%s+L</b> to focus the address bar</li>
				<li>Click on tabs to switch between them</li>
			</ul>
			<h3>Philosophy</h3>
			<p>In an age of <i>bloated browsers</i>, Nowser embraces <b>simplicity</b>. No extensions, no trackers, no complexity - just pure browsing.</p>
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
		"&mdash;":  "—",
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
		title = "Local: " + strings.TrimPrefix(filePath, "/")
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
	tabBarY := float32(0)
	tabWidth := float32(200)
	maxTabWidth := float32(900) / float32(len(app.tabs))
	if maxTabWidth < tabWidth {
		tabWidth = maxTabWidth
	}

	// Background for tab bar
	rl.DrawRectangle(0, 0, 900, int32(app.tabHeight), rl.Color{R: 240, G: 240, B: 240, A: 255})
	rl.DrawLine(0, int32(app.tabHeight), 900, int32(app.tabHeight), rl.Gray)

	currentX := float32(0)

	for i, tab := range app.tabs {
		isActive := i == app.activeTabIndex

		// Tab background
		tabColor := rl.Color{R: 220, G: 220, B: 220, A: 255}
		if isActive {
			tabColor = rl.White
		}

		tabRect := rl.NewRectangle(currentX, tabBarY, tabWidth, app.tabHeight)
		rl.DrawRectangleRec(tabRect, tabColor)

		// Tab border
		if isActive {
			rl.DrawRectangleLinesEx(tabRect, 1, rl.Gray)
		} else {
			rl.DrawLine(int32(currentX+tabWidth), int32(tabBarY), int32(currentX+tabWidth), int32(app.tabHeight), rl.Gray)
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

		rl.DrawText(title, int32(currentX+8), int32(tabBarY+10), 10, textColor)

		// Close button (X)
		if len(app.tabs) > 1 {
			closeX := currentX + tabWidth - 20
			closeY := tabBarY + 10
			closeColor := rl.Gray

			// Check if mouse is over close button
			mousePos := rl.GetMousePosition()
			closeRect := rl.NewRectangle(closeX, closeY, 12, 12)
			if rl.CheckCollisionPointRec(mousePos, closeRect) {
				closeColor = rl.Red
				rl.SetMouseCursor(rl.MouseCursorPointingHand)

				// Handle close click
				if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
					app.closeTab(i)
					return
				}
			}

			rl.DrawText("×", int32(closeX), int32(closeY), 10, closeColor)
		}

		// Handle tab click
		mousePos := rl.GetMousePosition()
		if rl.CheckCollisionPointRec(mousePos, tabRect) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			// Don't switch tabs if clicking close button
			closeX := currentX + tabWidth - 20
			if mousePos.X < closeX {
				app.activeTabIndex = i
				app.editingAddress = false
			}
		}

		currentX += tabWidth
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

	barY := app.tabHeight
	barHeight := app.addressBarHeight

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

	// Handle address bar click
	mousePos := rl.GetMousePosition()
	if rl.CheckCollisionPointRec(mousePos, barRect) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
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

	contentY := app.tabHeight + app.addressBarHeight
	contentHeight := 700 - contentY - 30 // Leave space for status bar

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

	statusY := float32(670)

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
	hints := fmt.Sprintf("%s+T: New Tab | %s+W: Close | %s+L: Address | %s+R: Refresh | Esc: Quit",
		app.modifierKey, app.modifierKey, app.modifierKey, app.modifierKey)
	hintsWidth := rl.MeasureText(hints, 10)
	rl.DrawText(hints, 900-hintsWidth-10, int32(statusY+10), 10, rl.Gray)
}

// Handle keyboard shortcuts
func (app *BrowserApp) handleKeyboard() {
	// Global shortcuts with platform-appropriate modifier
	if app.isModifierPressed() {
		if rl.IsKeyPressed(rl.KeyT) {
			// New tab
			app.createNewTab("https://", false)
		}
		if rl.IsKeyPressed(rl.KeyW) {
			// Close tab
			if len(app.tabs) > 1 {
				app.closeTab(app.activeTabIndex)
			}
		}
		if rl.IsKeyPressed(rl.KeyL) {
			// Focus address bar
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

	// Handle address bar input
	if app.editingAddress {
		app.handleAddressBarInput()
	}
}

// Update application state
func (app *BrowserApp) update() {
	// Reset mouse cursor
	rl.SetMouseCursor(rl.MouseCursorDefault)

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
		}
	}

	// Handle keyboard input
	app.handleKeyboard()

	// Update current tab widget
	tab := app.currentTab()
	if tab != nil && tab.Widget != nil && !app.editingAddress {
		tab.Widget.Update()
	}

	// Click outside address bar to stop editing
	if app.editingAddress {
		mousePos := rl.GetMousePosition()
		addressRect := rl.NewRectangle(10, app.tabHeight+5, 880, 30)
		if !rl.CheckCollisionPointRec(mousePos, addressRect) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			app.editingAddress = false
		}
	}
}

// Cleanup resources
func (app *BrowserApp) cleanup() {
	for _, tab := range app.tabs {
		if tab.Widget != nil {
			tab.Widget.Unload()
		}
	}
}

func main() {
	rl.InitWindow(900, 700, "The Nowser Antiexplorer")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)
	rl.SetConfigFlags(rl.FlagWindowResizable)

	// Initialize browser
	app := NewBrowserApp()
	defer app.cleanup()

	// Load URL from command line if provided, otherwise check for index.html
	if len(os.Args) > 1 {
		app.loadURL(os.Args[1])
	}
	// Note: index.html loading is handled in NewBrowserApp()

	// Main loop
	for !rl.WindowShouldClose() {
		app.update()

		rl.BeginDrawing()
		rl.ClearBackground(rl.Color{R: 245, G: 245, B: 245, A: 255})

		app.renderTabBar()
		app.renderAddressBar()
		app.renderContent()
		app.renderStatusBar()

		rl.EndDrawing()
	}
}
