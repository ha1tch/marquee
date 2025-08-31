package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ha1tch/marquee"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// MarqueeDownApp represents the main application state
type MarqueeDownApp struct {
	widget          *marquee.HTMLWidget
	currentFile     string
	lastModTime     time.Time
	statusMessage   string
	showFileDialog  bool
	fileList        []string
	selectedFileIdx int
	searchPath      string
}

// Enhanced markdown to HTML converter with more features
func markdownToHTML(markdown string) string {
	html := markdown
	
	// Convert headings (support for # ## ### #### ##### ######)
	html = regexp.MustCompile(`(?m)^###### (.*?)$`).ReplaceAllString(html, "<h6>$1</h6>")
	html = regexp.MustCompile(`(?m)^##### (.*?)$`).ReplaceAllString(html, "<h5>$1</h5>")
	html = regexp.MustCompile(`(?m)^#### (.*?)$`).ReplaceAllString(html, "<h4>$1</h4>")
	html = regexp.MustCompile(`(?m)^### (.*?)$`).ReplaceAllString(html, "<h3>$1</h3>")
	html = regexp.MustCompile(`(?m)^## (.*?)$`).ReplaceAllString(html, "<h2>$1</h2>")
	html = regexp.MustCompile(`(?m)^# (.*?)$`).ReplaceAllString(html, "<h1>$1</h1>")
	
	// Convert code blocks (triple backticks) to paragraphs with italic formatting
	html = regexp.MustCompile("(?s)```[a-zA-Z]*\n(.*?)\n```").ReplaceAllString(html, "<p><i>$1</i></p>")
	
	// Convert inline code (single backticks) to italic
	html = regexp.MustCompile("`([^`]+)`").ReplaceAllString(html, "<i>$1</i>")
	
	// Convert bold and italic (order matters - bold first to avoid conflicts)
	html = regexp.MustCompile(`\*\*\*(.*?)\*\*\*`).ReplaceAllString(html, "<b><i>$1</i></b>") // Bold italic
	html = regexp.MustCompile(`\*\*(.*?)\*\*`).ReplaceAllString(html, "<b>$1</b>")           // Bold
	html = regexp.MustCompile(`\*(.*?)\*`).ReplaceAllString(html, "<i>$1</i>")              // Italic
	
	// Convert links
	html = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(html, `<a href="$2">$1</a>`)
	
	// Convert horizontal rules
	html = regexp.MustCompile(`(?m)^---+$`).ReplaceAllString(html, "<hr>")
	html = regexp.MustCompile(`(?m)^\*\*\*+$`).ReplaceAllString(html, "<hr>")
	
	// Convert line breaks (double newlines become paragraph breaks)
	html = regexp.MustCompile(`\n\n+`).ReplaceAllString(html, "\n\n")
	
	// Process line by line for lists and paragraphs
	lines := strings.Split(html, "\n")
	var result []string
	inUnorderedList := false
	inOrderedList := false
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines in lists
		if trimmed == "" {
			if !inUnorderedList && !inOrderedList {
				result = append(result, "")
			}
			continue
		}
		
		// Handle unordered lists
		if regexp.MustCompile(`^[-*+] `).MatchString(trimmed) {
			if !inUnorderedList {
				if inOrderedList {
					result = append(result, "</ol>")
					inOrderedList = false
				}
				result = append(result, "<ul>")
				inUnorderedList = true
			}
			content := regexp.MustCompile(`^[-*+] `).ReplaceAllString(trimmed, "")
			result = append(result, "<li>"+content+"</li>")
			continue
		}
		
		// Handle ordered lists
		if regexp.MustCompile(`^\d+\. `).MatchString(trimmed) {
			if !inOrderedList {
				if inUnorderedList {
					result = append(result, "</ul>")
					inUnorderedList = false
				}
				result = append(result, "<ol>")
				inOrderedList = true
			}
			content := regexp.MustCompile(`^\d+\. `).ReplaceAllString(trimmed, "")
			result = append(result, "<li>"+content+"</li>")
			continue
		}
		
		// Close any open lists
		if inUnorderedList {
			result = append(result, "</ul>")
			inUnorderedList = false
		}
		if inOrderedList {
			result = append(result, "</ol>")
			inOrderedList = false
		}
		
		// Handle headings (already converted) and other content
		if strings.HasPrefix(trimmed, "<h") || strings.HasPrefix(trimmed, "<hr>") {
			result = append(result, trimmed)
		} else if trimmed != "" {
			// Check if next line would start a heading to avoid wrapping in <p>
			nextLineIsHeading := false
			if i+1 < len(lines) {
				nextTrimmed := strings.TrimSpace(lines[i+1])
				nextLineIsHeading = strings.HasPrefix(nextTrimmed, "<h")
			}
			
			if !nextLineIsHeading {
				result = append(result, "<p>"+trimmed+"</p>")
			} else {
				result = append(result, trimmed)
			}
		}
	}
	
	// Close any remaining lists
	if inUnorderedList {
		result = append(result, "</ul>")
	}
	if inOrderedList {
		result = append(result, "</ol>")
	}
	
	return strings.Join(result, "\n")
}

// Get list of markdown files in current directory
func getMarkdownFiles(path string) []string {
	var files []string
	
	entries, err := os.ReadDir(path)
	if err != nil {
		return files
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".md" || ext == ".markdown" || ext == ".txt" {
				files = append(files, name)
			}
		}
	}
	
	sort.Strings(files)
	return files
}

// Check if file has been modified
func fileModified(filename string, lastMod time.Time) bool {
	if filename == "" {
		return false
	}
	
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	
	return info.ModTime().After(lastMod)
}

// Load and convert markdown file
func (app *MarqueeDownApp) loadFile(filename string) {
	if filename == "" {
		return
	}
	
	// Update window title
	rl.SetWindowTitle(fmt.Sprintf("MarqueeDown - %s", filepath.Base(filename)))
	
	// Read file
	content, err := os.ReadFile(filename)
	if err != nil {
		errorHTML := fmt.Sprintf(`
			<h1>Error Loading File</h1>
			<p><b>Could not read:</b> <i>%s</i></p>
			<p><b>Error:</b> %s</p>
			<hr>
			<p>Press <b>Ctrl+O</b> to open a different file</p>
		`, filename, err.Error())
		
		if app.widget != nil {
			app.widget.Unload()
		}
		app.widget = marquee.NewHTMLWidget(errorHTML)
		app.statusMessage = fmt.Sprintf("Error: %s", err.Error())
		return
	}
	
	// Convert markdown to HTML
	htmlContent := markdownToHTML(string(content))
	
	// Update widget
	if app.widget != nil {
		app.widget.Unload()
	}
	app.widget = marquee.NewHTMLWidget(htmlContent)
	
	// Update tracking info
	app.currentFile = filename
	info, _ := os.Stat(filename)
	if info != nil {
		app.lastModTime = info.ModTime()
	}
	
	app.statusMessage = fmt.Sprintf("Loaded: %s (%d bytes)", filepath.Base(filename), len(content))
}

// Render file selection dialog
func (app *MarqueeDownApp) renderFileDialog() {
	// Semi-transparent overlay
	rl.DrawRectangle(0, 0, 900, 700, rl.Color{R: 0, G: 0, B: 0, A: 128})
	
	// Dialog box
	dialogWidth := float32(500)
	dialogHeight := float32(400)
	dialogX := (900 - dialogWidth) / 2
	dialogY := (700 - dialogHeight) / 2
	
	rl.DrawRectangle(int32(dialogX), int32(dialogY), int32(dialogWidth), int32(dialogHeight), rl.White)
	rl.DrawRectangleLinesEx(rl.NewRectangle(dialogX, dialogY, dialogWidth, dialogHeight), 2, rl.DarkGray)
	
	// Title
	rl.DrawText("Select Markdown File", int32(dialogX+20), int32(dialogY+20), 20, rl.Black)
	
	// File list
	listY := dialogY + 60
	for i, file := range app.fileList {
		color := rl.Black
		if i == app.selectedFileIdx {
			// Highlight selected file
			rl.DrawRectangle(int32(dialogX+10), int32(listY-2), int32(dialogWidth-20), 25, rl.LightGray)
			color = rl.DarkBlue
		}
		
		rl.DrawText(file, int32(dialogX+20), int32(listY), 16, color)
		listY += 25
		
		// Don't draw beyond dialog bounds
		if listY > dialogY+dialogHeight-80 {
			break
		}
	}
	
	// Instructions
	instructY := dialogY + dialogHeight - 60
	rl.DrawText("Up/Down: Navigate | Enter: Open | Esc: Cancel", int32(dialogX+20), int32(instructY), 12, rl.DarkGray)
}

// Handle file dialog input
func (app *MarqueeDownApp) handleFileDialogInput() {
	// Navigation
	if rl.IsKeyPressed(rl.KeyUp) && app.selectedFileIdx > 0 {
		app.selectedFileIdx--
	}
	if rl.IsKeyPressed(rl.KeyDown) && app.selectedFileIdx < len(app.fileList)-1 {
		app.selectedFileIdx++
	}
	
	// Selection
	if rl.IsKeyPressed(rl.KeyEnter) && len(app.fileList) > 0 {
		selectedFile := app.fileList[app.selectedFileIdx]
		app.loadFile(selectedFile)
		app.showFileDialog = false
	}
	
	// Cancel
	if rl.IsKeyPressed(rl.KeyEscape) {
		app.showFileDialog = false
	}
}

func main() {
	rl.InitWindow(900, 700, "MarqueeDown - Markdown Viewer")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)
	
	// Initialize application
	app := &MarqueeDownApp{
		searchPath: ".",
		statusMessage: "Press Ctrl+O to open a file, or drop a .md file to load it",
	}
	
	// Load file from command line argument if provided
	if len(os.Args) > 1 {
		app.loadFile(os.Args[1])
	} else {
		// Create welcome content
		welcomeHTML := `
			<h1>MarqueeDown</h1>
			<p>A lightweight Markdown viewer built with <b>MARQUEE</b></p>
			<hr>
			<h2>Getting Started</h2>
			<ul>
				<li>Press <b>Ctrl+O</b> to open a markdown file</li>
				<li>Press <b>F5</b> to refresh the current file</li>
				<li>Press <b>Esc</b> to quit</li>
			</ul>
			<h3>Features</h3>
			<ul>
				<li><b>Live reload</b> - automatically refreshes when file changes</li>
				<li><i>Enhanced markdown parsing</i> - supports more syntax</li>
				<li>File browser with keyboard navigation</li>
				<li>Cross-platform font rendering</li>
			</ul>
			<p>Drop a <i>.md</i> file here or use <b>Ctrl+O</b> to browse files!</p>
		`
		app.widget = marquee.NewHTMLWidget(welcomeHTML)
	}
	
	defer func() {
		if app.widget != nil {
			app.widget.Unload()
		}
	}()
	
	// Enable file dropping
	rl.SetConfigFlags(rl.FlagWindowResizable)
	
	// Main loop
	for !rl.WindowShouldClose() {
		// Handle keyboard shortcuts
		if rl.IsKeyDown(rl.KeyLeftControl) {
			if rl.IsKeyPressed(rl.KeyO) {
				// Open file dialog
				app.fileList = getMarkdownFiles(app.searchPath)
				if len(app.fileList) > 0 {
					app.showFileDialog = true
					app.selectedFileIdx = 0
				} else {
					app.statusMessage = "No markdown files found in current directory"
				}
			}
		}
		
		if rl.IsKeyPressed(rl.KeyF5) {
			// Refresh current file
			if app.currentFile != "" {
				app.loadFile(app.currentFile)
			}
		}
		
		if rl.IsKeyPressed(rl.KeyEscape) {
			if app.showFileDialog {
				app.showFileDialog = false
			} else {
				break // Quit application
			}
		}
		
		// Handle file drops
		if rl.IsFileDropped() {
			droppedFiles := rl.LoadDroppedFiles()
			if droppedFiles.Count > 0 {
				filename := rl.GetDroppedFileNames(droppedFiles)[0]
				ext := strings.ToLower(filepath.Ext(filename))
				if ext == ".md" || ext == ".markdown" || ext == ".txt" {
					app.loadFile(filename)
				} else {
					app.statusMessage = "Please drop a .md, .markdown, or .txt file"
				}
			}
			rl.UnloadDroppedFiles(droppedFiles)
		}
		
		// Auto-reload if file changed
		if app.currentFile != "" && fileModified(app.currentFile, app.lastModTime) {
			time.Sleep(100 * time.Millisecond) // Brief delay to avoid partial reads
			app.loadFile(app.currentFile)
			app.statusMessage = fmt.Sprintf("Auto-reloaded: %s", filepath.Base(app.currentFile))
		}
		
		// Handle file dialog input
		if app.showFileDialog {
			app.handleFileDialogInput()
		} else if app.widget != nil {
			app.widget.Update()
		}
		
		// Render
		rl.BeginDrawing()
		rl.ClearBackground(rl.Color{R: 245, G: 245, B: 245, A: 255}) // Light background
		
		// Render main content
		if app.widget != nil {
			app.widget.Render(20, 20, 860, 620)
		}
		
		// Render status bar
		statusBarY := float32(650)
		rl.DrawRectangle(0, int32(statusBarY), 900, 50, rl.Color{R: 230, G: 230, B: 230, A: 255})
		rl.DrawLine(0, int32(statusBarY), 900, int32(statusBarY), rl.Gray)
		
		// Status text
		rl.DrawText(app.statusMessage, 10, int32(statusBarY+15), 12, rl.DarkGray)
		
		// Keyboard shortcuts hint
		hintsText := "Ctrl+O: Open | F5: Refresh | Esc: Quit"
		hintsWidth := rl.MeasureText(hintsText, 12)
		rl.DrawText(hintsText, 900-hintsWidth-10, int32(statusBarY+15), 12, rl.DarkGray)
		
		// Render file dialog on top
		if app.showFileDialog {
			app.renderFileDialog()
		}
		
		rl.EndDrawing()
	}
}