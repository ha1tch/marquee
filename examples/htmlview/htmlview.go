package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ha1tch/marquee"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// HTMLViewApp represents the main application state
type HTMLViewApp struct {
	widget          *marquee.HTMLWidget
	currentFile     string
	lastModTime     time.Time
	statusMessage   string
	showFileDialog  bool
	fileList        []string
	selectedFileIdx int
	searchPath      string
}

// Get list of HTML files in current directory
func getHTMLFiles(path string) []string {
	var files []string
	
	entries, err := os.ReadDir(path)
	if err != nil {
		return files
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".html" || ext == ".htm" || ext == ".txt" {
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

// Load HTML file
func (app *HTMLViewApp) loadFile(filename string) {
	if filename == "" {
		return
	}
	
	// Update window title
	rl.SetWindowTitle(fmt.Sprintf("HTML Viewer - %s", filepath.Base(filename)))
	
	// Read file
	content, err := os.ReadFile(filename)
	if err != nil {
		errorHTML := fmt.Sprintf(`
			<h1>Error Loading File</h1>
			<p><b>Could not read:</b> <i>%s</i></p>
			<p><b>Error:</b> %s</p>
			<hr>
			<p>Press <b>Ctrl+O</b> to open a different file or drag & drop an HTML file</p>
		`, filename, err.Error())
		
		if app.widget != nil {
			app.widget.Unload()
		}
		app.widget = marquee.NewHTMLWidget(errorHTML)
		app.statusMessage = fmt.Sprintf("Error: %s", err.Error())
		return
	}
	
	// Create widget with HTML content
	if app.widget != nil {
		app.widget.Unload()
	}
	app.widget = marquee.NewHTMLWidget(string(content))
	
	// Update tracking info
	app.currentFile = filename
	info, _ := os.Stat(filename)
	if info != nil {
		app.lastModTime = info.ModTime()
	}
	
	app.statusMessage = fmt.Sprintf("Loaded: %s (%d bytes)", filepath.Base(filename), len(content))
}

// Render file selection dialog
func (app *HTMLViewApp) renderFileDialog() {
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
	rl.DrawText("Select HTML File", int32(dialogX+20), int32(dialogY+20), 20, rl.Black)
	
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
func (app *HTMLViewApp) handleFileDialogInput() {
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
	rl.InitWindow(900, 700, "HTML Viewer")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)
	
	// Enable file dropping and window resizing
	rl.SetConfigFlags(rl.FlagWindowResizable)
	
	// Initialize application
	app := &HTMLViewApp{
		searchPath: ".",
		statusMessage: "Press Ctrl+O to open a file, or drag & drop an HTML file to load it",
	}
	
	// Load file from command line argument if provided
	if len(os.Args) > 1 {
		app.loadFile(os.Args[1])
	} else {
		// Create welcome content
		welcomeHTML := `
			<h1>HTML Viewer</h1>
			<p>A lightweight HTML viewer built with <b>MARQUEE</b></p>
			<hr>
			<h2>Features</h2>
			<ul>
				<li><b>Fast rendering</b> with native fonts</li>
				<li><i>Live reload</i> - automatically refreshes when files change</li>
				<li>Drag & drop support for HTML files</li>
				<li>Keyboard shortcuts for easy navigation</li>
				<li>Clickable hyperlinks with hover effects</li>
			</ul>
			<h3>Supported HTML Tags</h3>
			<ul>
				<li>Headings: <b>h1</b> through <b>h6</b></li>
				<li>Text formatting: <b>bold</b> and <i>italic</i></li>
				<li>Links: <a href="https://github.com/ha1tch/marquee">github.com/ha1tch/marquee</a></li>
				<li>Lists: both ordered and unordered</li>
				<li>Horizontal rules and line breaks</li>
			</ul>
			<hr>
			<h4>Quick Start</h4>
			<ol>
				<li>Press <b>Ctrl+O</b> to open an HTML file</li>
				<li>Or drag & drop an <i>.html</i> file into this window</li>
				<li>Press <b>F5</b> to refresh the current file</li>
				<li>Press <b>Esc</b> to quit</li>
			</ol>
			<p>Perfect for viewing documentation, help files, or any simple HTML content!</p>
		`
		app.widget = marquee.NewHTMLWidget(welcomeHTML)
	}
	
	defer func() {
		if app.widget != nil {
			app.widget.Unload()
		}
	}()
	
	// Main loop
	for !rl.WindowShouldClose() {
		// Handle keyboard shortcuts
		if rl.IsKeyDown(rl.KeyLeftControl) {
			if rl.IsKeyPressed(rl.KeyO) {
				// Open file dialog
				app.fileList = getHTMLFiles(app.searchPath)
				if len(app.fileList) > 0 {
					app.showFileDialog = true
					app.selectedFileIdx = 0
				} else {
					app.statusMessage = "No HTML files found in current directory"
				}
			}
		}
		
		if rl.IsKeyPressed(rl.KeyF5) {
			// Refresh current file
			if app.currentFile != "" {
				app.loadFile(app.currentFile)
				app.statusMessage = fmt.Sprintf("Refreshed: %s", filepath.Base(app.currentFile))
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
				if ext == ".html" || ext == ".htm" || ext == ".txt" {
					app.loadFile(filename)
				} else {
					app.statusMessage = "Please drop an .html, .htm, or .txt file"
				}
			}
			rl.UnloadDroppedFiles(droppedFiles)
		}
		
		// Auto-reload if file changed (with small delay to avoid partial reads)
		if app.currentFile != "" && fileModified(app.currentFile, app.lastModTime) {
			time.Sleep(100 * time.Millisecond)
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
		hintsText := "Ctrl+O: Open | F5: Refresh | Esc: Quit | Drag & Drop supported"
		hintsWidth := rl.MeasureText(hintsText, 12)
		rl.DrawText(hintsText, 900-hintsWidth-10, int32(statusBarY+15), 12, rl.DarkGray)
		
		// Render file dialog on top
		if app.showFileDialog {
			app.renderFileDialog()
		}
		
		rl.EndDrawing()
	}
}