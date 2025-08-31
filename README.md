# MARQUEE

A lightweight HTML rendering widget for raylib-go applications and games. MARQUEE provides immediate-mode rendering of basic HTML content using TTF fonts, perfect for displaying documentation, help systems, and rich text in games and desktop applications without the overhead of embedding a web browser.

## Why MARQUEE?

Modern applications often need to display formatted documentation or help content, but the available options are problematic:

- **Electron/Web browsers**: 100+ MB dependency for simple text rendering
- **Platform webviews**: Inconsistent behavior, security concerns, still heavyweight  
- **Native rich text controls**: Platform-specific, limited cross-platform support
- **Plain text**: Functional but lacks basic formatting for readable documentation

MARQUEE solves this by providing just enough HTML rendering for documentation needs while keeping dependencies minimal and performance high.

## Features

### Supported HTML Tags

- **Headings**: `<h1>` through `<h6>` with proper font sizing
- **Paragraphs**: `<p>` with automatic word wrapping
- **Text formatting**: `<b>` (bold) and `<i>` (italic)
- **Hyperlinks**: `<a href="...">` with hover effects and click handling
- **Lists**: Both `<ul>` (unordered) and `<ol>` (ordered) with `<li>` items
- **Separators**: `<hr>` horizontal rules
- **Line breaks**: `<br>` tags

### Advanced Features

- **Cross-platform font loading** with automatic fallbacks
- **Smooth scrolling** with fade-in/fade-out scrollbars
- **Clickable hyperlinks** with hover states and cursor changes
- **Mixed inline formatting** (bold, italic, and links within paragraphs)
- **Automatic text wrapping** and layout calculation
- **File-based content loading** for easy content management

## Installation

```bash
go get github.com/gen2brain/raylib-go/raylib
go get github.com/ha1tch/marquee
```

## Quick Start

```go
package main

import (
    "github.com/ha1tch/marquee"
    rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
    rl.InitWindow(900, 700, "My App with Help")
    defer rl.CloseWindow()

    // Load HTML content from file
    widget := marquee.NewHTMLWidget(htmlContent)
    defer widget.Unload()

    for !rl.WindowShouldClose() {
        widget.Update()
        
        rl.BeginDrawing()
        rl.ClearBackground(rl.LightGray)
        
        // Render HTML content in a 800x650 area at position (20,20)
        widget.Render(20, 20, 800, 650)
        
        rl.EndDrawing()
    }
}
```

## Usage Examples

### Loading from File

```go
// Load HTML from file
htmlBytes, err := os.ReadFile("documentation.html")
if err != nil {
    log.Fatal(err)
}

widget := marquee.NewHTMLWidget(string(htmlBytes))
defer widget.Unload()
```

### Inline HTML Content

```go
htmlContent := `
<h1>User Guide</h1>
<p>Welcome to <b>MyApp</b>! Here's how to get started:</p>
<ol>
    <li>Open a file using <i>File → Open</i></li>
    <li>Edit your content</li>
    <li>Save when finished</li>
</ol>
<p>For more help, visit <a href="https://myapp.com/docs">our documentation</a>.</p>
`

widget := marquee.NewHTMLWidget(htmlContent)
```

### Handling Link Clicks

```go
// Modify widget.Update() to use a callback instead of fmt.Printf:
// Replace: fmt.Printf("Clicked link: %s\n", area.URL)
// With: if onLinkClick != nil { onLinkClick(area.URL) }

// Then in your application:
widget := marquee.NewHTMLWidget(htmlContent)
widget.OnLinkClick = func(url string) {
    exec.Command("open", url).Start() // Open in default browser
}
```

## API Reference

### HTMLWidget

#### marquee.NewHTMLWidget(content string) *marquee.HTMLWidget
Creates a new HTML widget with the specified content.

#### Update()
Handles user input (scrolling, link interactions). Call once per frame.

#### Render(x, y, width, height float32)
Renders the HTML content within the specified bounds. Call once per frame after Update().

#### Unload()
Cleans up font resources. Call when the widget is no longer needed.

### HTMLElement

Represents a parsed HTML element with support for:
- Tag type and content
- Link URLs (`Href`)
- Heading levels (`Level`)
- Text formatting flags (`Bold`, `Italic`)
- Child elements for nested structures

## Font Support

MARQUEE automatically loads system fonts:

- **macOS**: Arial family from `/System/Library/Fonts/Supplemental/`
- **Windows**: Arial family from `C:/Windows/Fonts/`
- **Linux**: Liberation Sans from `/usr/share/fonts/truetype/liberation/`

Fonts are loaded at multiple sizes for headings (16px for body text, up to 32px for h1).

## Limitations

MARQUEE is intentionally minimal and does **not** support:

- CSS styling or external stylesheets
- JavaScript execution
- Images, videos, or multimedia content
- Complex layout (flexbox, grid, floats)
- Forms or input elements
- Nested formatting (e.g., bold italic text)
- Custom colors or themes (uses fixed color scheme)

These limitations keep the codebase small and focused on the core use case of documentation rendering.

## Technical Details

- **Parsing**: Uses regex-based HTML parsing optimized for the supported tag subset
- **Rendering**: Immediate-mode rendering recalculates layout each frame
- **Memory**: Minimal memory footprint, no DOM tree persistence
- **Performance**: Suitable for documents up to several pages; very large documents may impact framerate
- **Dependencies**: Only requires [raylib-go](https://github.com/gen2brain/raylib-go)

## Running the Examples

```bash
# HTML Viewer - compile and run with default example.html
go run examples/htmlview/htmlview.go

# Or specify a custom HTML file
go run examples/htmlview/htmlview.go my-documentation.html

# MarqueeDown - compile and run with default README.md
go run examples/marqueedown/marqueedown.go

# Or specify a custom Markdown file
go run examples/marqueedown/marqueedown.go my-document.md
```

The demo loads HTML content from a file and displays it in a scrollable widget with working links and formatting.

### Markdown Viewer Demo

Here's a simple example showing how to build a Markdown viewer on top of MARQUEE:

```go
package main

import (
    "os"
    "regexp"
    "strings"
    rl "github.com/gen2brain/raylib-go/raylib"
)

// Simple markdown to HTML converter
func markdownToHTML(markdown string) string {
    html := markdown
    
    // Convert headings
    html = regexp.MustCompile(`(?m)^### (.*?)# MARQUEE

A lightweight HTML rendering widget for Raylib applications. MARQUEE provides immediate-mode rendering of basic HTML content using TTF fonts, perfect for displaying documentation, help systems, and rich text in games and desktop applications without the overhead of embedding a web browser.

## Why MARQUEE?

Modern applications often need to display formatted documentation or help content, but the available options are problematic:

- **Electron/Web browsers**: 100+ MB dependency for simple text rendering
- **Platform webviews**: Inconsistent behavior, security concerns, still heavyweight  
- **Native rich text controls**: Platform-specific, limited cross-platform support
- **Plain text**: Functional but lacks basic formatting for readable documentation

MARQUEE solves this by providing just enough HTML rendering for documentation needs while keeping dependencies minimal and performance high.

## Features

### Supported HTML Tags

- **Headings**: `<h1>` through `<h6>` with proper font sizing
- **Paragraphs**: `<p>` with automatic word wrapping
- **Text formatting**: `<b>` (bold) and `<i>` (italic)
- **Hyperlinks**: `<a href="...">` with hover effects and click handling
- **Lists**: Both `<ul>` (unordered) and `<ol>` (ordered) with `<li>` items
- **Separators**: `<hr>` horizontal rules
- **Line breaks**: `<br>` tags

### Advanced Features

- **Cross-platform font loading** with automatic fallbacks
- **Smooth scrolling** with fade-in/fade-out scrollbars
- **Clickable hyperlinks** with hover states and cursor changes
- **Mixed inline formatting** (bold, italic, and links within paragraphs)
- **Automatic text wrapping** and layout calculation
- **File-based content loading** for easy content management

## Installation

```bash
go get github.com/gen2brain/raylib-go/raylib
```

Copy `marquee.go` into your project.

## Quick Start

```go
package main

import (
    rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
    rl.InitWindow(900, 700, "My App with Help")
    defer rl.CloseWindow()

    // Load HTML content from file
    widget := NewHTMLWidget(htmlContent)
    defer widget.Unload()

    for !rl.WindowShouldClose() {
        widget.Update()
        
        rl.BeginDrawing()
        rl.ClearBackground(rl.LightGray)
        
        // Render HTML content in a 800x650 area at position (20,20)
        widget.Render(20, 20, 800, 650)
        
        rl.EndDrawing()
    }
}
```

## Usage Examples

### Loading from File

```go
// Load HTML from file
htmlBytes, err := os.ReadFile("documentation.html")
if err != nil {
    log.Fatal(err)
}

widget := NewHTMLWidget(string(htmlBytes))
defer widget.Unload()
```

### Inline HTML Content

```go
htmlContent := `
<h1>User Guide</h1>
<p>Welcome to <b>MyApp</b>! Here's how to get started:</p>
<ol>
    <li>Open a file using <i>File → Open</i></li>
    <li>Edit your content</li>
    <li>Save when finished</li>
</ol>
<p>For more help, visit <a href="https://myapp.com/docs">our documentation</a>.</p>
`

widget := NewHTMLWidget(htmlContent)
```

### Handling Link Clicks

Link clicks are automatically detected and printed to stdout. To handle them in your application:

```go
// In your Update() loop, after widget.Update():
// Link clicks are printed as: "Clicked link: [URL]"
// You can modify the widget code to call a custom callback instead
```

## API Reference

### HTMLWidget

#### NewHTMLWidget(content string) *HTMLWidget
Creates a new HTML widget with the specified content.

#### Update()
Handles user input (scrolling, link interactions). Call once per frame.

#### Render(x, y, width, height float32)
Renders the HTML content within the specified bounds. Call once per frame after Update().

#### Unload()
Cleans up font resources. Call when the widget is no longer needed.

### HTMLElement

Represents a parsed HTML element with support for:
- Tag type and content
- Link URLs (`Href`)
- Heading levels (`Level`)
- Text formatting flags (`Bold`, `Italic`)
- Child elements for nested structures

## Font Support

MARQUEE automatically loads system fonts:

- **macOS**: Arial family from `/System/Library/Fonts/Supplemental/`
- **Windows**: Arial family from `C:/Windows/Fonts/`
- **Linux**: Liberation Sans from `/usr/share/fonts/truetype/liberation/`

Fonts are loaded at multiple sizes for headings (16px for body text, up to 32px for h1).

## Limitations

MARQUEE is intentionally minimal and does **not** support:

- CSS styling or external stylesheets
- JavaScript execution
- Images, videos, or multimedia content
- Complex layout (flexbox, grid, floats)
- Forms or input elements
- Nested formatting (e.g., bold italic text)
- Custom colors or themes (uses fixed color scheme)

These limitations keep the codebase small and focused on the core use case of documentation rendering.

## Technical Details

- **Parsing**: Uses regex-based HTML parsing optimized for the supported tag subset
- **Rendering**: Immediate-mode rendering recalculates layout each frame
- **Memory**: Minimal memory footprint, no DOM tree persistence
- **Performance**: Suitable for documents up to several pages; very large documents may impact framerate
- **Dependencies**: Only requires [raylib-go](https://github.com/gen2brain/raylib-go)

## Example HTML Structure

```html
<h1>Documentation Title</h1>
<p>This is a paragraph with <b>bold text</b> and <i>italic text</i>.</p>

<h2>Features</h2>
<ul>
    <li>Simple and <b>lightweight</b></li>
    <li>Cross-platform font support</li>
    <li>Clickable links: <a href="https://example.com">example.com</a></li>
</ul>

<hr>

<h3>Getting Started</h3>
<ol>
    <li>Install dependencies</li>
    <li>Copy the widget code</li>
    <li>Create your HTML content</li>
</ol>
```

## Running the Demo

```bash
# Compile and run with default example.html
go run marquee.go

# Or specify a custom HTML file
go run marquee.go my-documentation.html
```

).ReplaceAllString(html, "<h3>$1</h3>")
    html = regexp.MustCompile(`(?m)^## (.*?)# MARQUEE

A lightweight HTML rendering widget for Raylib applications. MARQUEE provides immediate-mode rendering of basic HTML content using TTF fonts, perfect for displaying documentation, help systems, and rich text in games and desktop applications without the overhead of embedding a web browser.

## Why MARQUEE?

Modern applications often need to display formatted documentation or help content, but the available options are problematic:

- **Electron/Web browsers**: 100+ MB dependency for simple text rendering
- **Platform webviews**: Inconsistent behavior, security concerns, still heavyweight  
- **Native rich text controls**: Platform-specific, limited cross-platform support
- **Plain text**: Functional but lacks basic formatting for readable documentation

MARQUEE solves this by providing just enough HTML rendering for documentation needs while keeping dependencies minimal and performance high.

## Features

### Supported HTML Tags

- **Headings**: `<h1>` through `<h6>` with proper font sizing
- **Paragraphs**: `<p>` with automatic word wrapping
- **Text formatting**: `<b>` (bold) and `<i>` (italic)
- **Hyperlinks**: `<a href="...">` with hover effects and click handling
- **Lists**: Both `<ul>` (unordered) and `<ol>` (ordered) with `<li>` items
- **Separators**: `<hr>` horizontal rules
- **Line breaks**: `<br>` tags

### Advanced Features

- **Cross-platform font loading** with automatic fallbacks
- **Smooth scrolling** with fade-in/fade-out scrollbars
- **Clickable hyperlinks** with hover states and cursor changes
- **Mixed inline formatting** (bold, italic, and links within paragraphs)
- **Automatic text wrapping** and layout calculation
- **File-based content loading** for easy content management

## Installation

```bash
go get github.com/gen2brain/raylib-go/raylib
```

Copy `marquee.go` into your project.

## Quick Start

```go
package main

import (
    rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
    rl.InitWindow(900, 700, "My App with Help")
    defer rl.CloseWindow()

    // Load HTML content from file
    widget := NewHTMLWidget(htmlContent)
    defer widget.Unload()

    for !rl.WindowShouldClose() {
        widget.Update()
        
        rl.BeginDrawing()
        rl.ClearBackground(rl.LightGray)
        
        // Render HTML content in a 800x650 area at position (20,20)
        widget.Render(20, 20, 800, 650)
        
        rl.EndDrawing()
    }
}
```

## Usage Examples

### Loading from File

```go
// Load HTML from file
htmlBytes, err := os.ReadFile("documentation.html")
if err != nil {
    log.Fatal(err)
}

widget := NewHTMLWidget(string(htmlBytes))
defer widget.Unload()
```

### Inline HTML Content

```go
htmlContent := `
<h1>User Guide</h1>
<p>Welcome to <b>MyApp</b>! Here's how to get started:</p>
<ol>
    <li>Open a file using <i>File → Open</i></li>
    <li>Edit your content</li>
    <li>Save when finished</li>
</ol>
<p>For more help, visit <a href="https://myapp.com/docs">our documentation</a>.</p>
`

widget := NewHTMLWidget(htmlContent)
```

### Handling Link Clicks

Link clicks are automatically detected and printed to stdout. To handle them in your application:

```go
// In your Update() loop, after widget.Update():
// Link clicks are printed as: "Clicked link: [URL]"
// You can modify the widget code to call a custom callback instead
```

## API Reference

### HTMLWidget

#### NewHTMLWidget(content string) *HTMLWidget
Creates a new HTML widget with the specified content.

#### Update()
Handles user input (scrolling, link interactions). Call once per frame.

#### Render(x, y, width, height float32)
Renders the HTML content within the specified bounds. Call once per frame after Update().

#### Unload()
Cleans up font resources. Call when the widget is no longer needed.

### HTMLElement

Represents a parsed HTML element with support for:
- Tag type and content
- Link URLs (`Href`)
- Heading levels (`Level`)
- Text formatting flags (`Bold`, `Italic`)
- Child elements for nested structures

## Font Support

MARQUEE automatically loads system fonts:

- **macOS**: Arial family from `/System/Library/Fonts/Supplemental/`
- **Windows**: Arial family from `C:/Windows/Fonts/`
- **Linux**: Liberation Sans from `/usr/share/fonts/truetype/liberation/`

Fonts are loaded at multiple sizes for headings (16px for body text, up to 32px for h1).

## Limitations

MARQUEE is intentionally minimal and does **not** support:

- CSS styling or external stylesheets
- JavaScript execution
- Images, videos, or multimedia content
- Complex layout (flexbox, grid, floats)
- Forms or input elements
- Nested formatting (e.g., bold italic text)
- Custom colors or themes (uses fixed color scheme)

These limitations keep the codebase small and focused on the core use case of documentation rendering.

## Technical Details

- **Parsing**: Uses regex-based HTML parsing optimized for the supported tag subset
- **Rendering**: Immediate-mode rendering recalculates layout each frame
- **Memory**: Minimal memory footprint, no DOM tree persistence
- **Performance**: Suitable for documents up to several pages; very large documents may impact framerate
- **Dependencies**: Only requires [raylib-go](https://github.com/gen2brain/raylib-go)

## Running the Demo

```bash
# Compile and run with default example.html
go run marquee.go

# Or specify a custom HTML file
go run marquee.go my-documentation.html
```

### Supported HTML Tags

- **Headings**: `<h1>` through `<h6>` with proper font sizing
- **Paragraphs**: `<p>` with automatic word wrapping
- **Text formatting**: `<b>` (bold) and `<i>` (italic)
- **Hyperlinks**: `\<\a href="...">` with hover effects and click handling
- **Lists**: Both `<ul>` (unordered) and `<ol>` (ordered) with `<li>` items
- **Separators**: `<hr>` horizontal rules
- **Line breaks**: `<br>` tags

## Installation

```bash
go get github.com/gen2brain/raylib-go/raylib
```


## Quick Start

### Loading from File

```go
// Load HTML from file
htmlBytes, err := os.ReadFile("documentation.html")
if err != nil {
    log.Fatal(err)
}

widget := NewHTMLWidget(string(htmlBytes))
defer widget.Unload()
```

### Inline HTML Content

```go
htmlContent := `
<h1>User Guide</h1>
<p>Welcome to <b>MyApp</b>! Here's how to get started:</p>
<ol>
    <li>Open a file using <i>File → Open</i></li>
    <li>Edit your content</li>
    <li>Save when finished</li>
</ol>
<p>For more help, visit <a href="https://myapp.com/docs">our documentation</a>.</p>
`

widget := NewHTMLWidget(htmlContent)
```

### Handling Link Clicks

Link clicks are automatically detected and printed to stdout. To handle them in your application:

```go
// In your Update() loop, after widget.Update():
// Link clicks are printed as: "Clicked link: [URL]"
// You can modify the widget code to call a custom callback instead
```

## API Reference

### HTMLWidget

#### NewHTMLWidget(content string) *HTMLWidget
Creates a new HTML widget with the specified content.

#### Update()
Handles user input (scrolling, link interactions). Call once per frame.

#### Render(x, y, width, height float32)
Renders the HTML content within the specified bounds. Call once per frame after Update().

#### Unload()
Cleans up font resources. Call when the widget is no longer needed.

### HTMLElement

Represents a parsed HTML element with support for:
- Tag type and content
- Link URLs (`Href`)
- Heading levels (`Level`)
- Text formatting flags (`Bold`, `Italic`)
- Child elements for nested structures

## Font Support

MARQUEE automatically loads system fonts:

- **macOS**: Arial family from `/System/Library/Fonts/Supplemental/`
- **Windows**: Arial family from `C:/Windows/Fonts/`
- **Linux**: Liberation Sans from `/usr/share/fonts/truetype/liberation/`

If these are not found, MARQUEE will use the simple internal font for rendering text.
Fonts are loaded at multiple sizes for headings (16px for body text, up to 32px for h1).

## Limitations

MARQUEE is intentionally minimal and does **not** support:

- CSS styling or external stylesheets
- JavaScript execution
- Images, videos, or multimedia content
- Complex layout (flexbox, grid, floats)
- Forms or input elements
- Nested formatting (e.g., bold italic text)
- Custom colors or themes (uses fixed color scheme)

These limitations keep the codebase small and focused on the core use case of documentation rendering.

## Technical Details

- **Parsing**: Uses regex-based HTML parsing optimized for the supported tag subset
- **Rendering**: Immediate-mode rendering recalculates layout each frame
- **Memory**: Minimal memory footprint, no DOM tree persistence
- **Performance**: Suitable for documents up to several pages; very large documents may impact framerate
- **Dependencies**: Only requires [raylib-go](https://github.com/gen2brain/raylib-go)


## Running the Demo

```bash
# Compile and run with default example.html
go run marquee.go

# Or specify a custom HTML file
go run marquee.go my-documentation.html
```

).ReplaceAllString(html, "<h1>$1</h1>")
    
    // Convert bold and italic
    html = regexp.MustCompile(`\*\*(.*?)\*\*`).ReplaceAllString(html, "<b>$1</b>")
    html = regexp.MustCompile(`\*(.*?)\*`).ReplaceAllString(html, "<i>$1</i>")
    
    // Convert links
    html = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(html, `<a href="$2">$1</a>`)
    
    // Convert horizontal rules
    html = regexp.MustCompile(`(?m)^---+# MARQUEE

A lightweight HTML rendering widget for Raylib applications. MARQUEE provides immediate-mode rendering of basic HTML content using TTF fonts, perfect for displaying documentation, help systems, and rich text in games and desktop applications without the overhead of embedding a web browser.

## Why MARQUEE?

Modern applications often need to display formatted documentation or help content, but the available options are problematic:

- **Electron/Web browsers**: 100+ MB dependency for simple text rendering
- **Platform webviews**: Inconsistent behavior, security concerns, still heavyweight  
- **Native rich text controls**: Platform-specific, limited cross-platform support
- **Plain text**: Functional but lacks basic formatting for readable documentation

MARQUEE solves this by providing just enough HTML rendering for documentation needs while keeping dependencies minimal and performance high.

## Features

### Supported HTML Tags

- **Headings**: `<h1>` through `<h6>` with proper font sizing
- **Paragraphs**: `<p>` with automatic word wrapping
- **Text formatting**: `<b>` (bold) and `<i>` (italic)
- **Hyperlinks**: `<a href="...">` with hover effects and click handling
- **Lists**: Both `<ul>` (unordered) and `<ol>` (ordered) with `<li>` items
- **Separators**: `<hr>` horizontal rules
- **Line breaks**: `<br>` tags

### Advanced Features

- **Cross-platform font loading** with automatic fallbacks
- **Smooth scrolling** with fade-in/fade-out scrollbars
- **Clickable hyperlinks** with hover states and cursor changes
- **Mixed inline formatting** (bold, italic, and links within paragraphs)
- **Automatic text wrapping** and layout calculation
- **File-based content loading** for easy content management

## Installation

```bash
go get github.com/gen2brain/raylib-go/raylib
```

Copy `marquee.go` into your project.

## Quick Start

```go
package main

import (
    rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
    rl.InitWindow(900, 700, "My App with Help")
    defer rl.CloseWindow()

    // Load HTML content from file
    widget := NewHTMLWidget(htmlContent)
    defer widget.Unload()

    for !rl.WindowShouldClose() {
        widget.Update()
        
        rl.BeginDrawing()
        rl.ClearBackground(rl.LightGray)
        
        // Render HTML content in a 800x650 area at position (20,20)
        widget.Render(20, 20, 800, 650)
        
        rl.EndDrawing()
    }
}
```

## Usage Examples

### Loading from File

```go
// Load HTML from file
htmlBytes, err := os.ReadFile("documentation.html")
if err != nil {
    log.Fatal(err)
}

widget := NewHTMLWidget(string(htmlBytes))
defer widget.Unload()
```

### Inline HTML Content

```go
htmlContent := `
<h1>User Guide</h1>
<p>Welcome to <b>MyApp</b>! Here's how to get started:</p>
<ol>
    <li>Open a file using <i>File → Open</i></li>
    <li>Edit your content</li>
    <li>Save when finished</li>
</ol>
<p>For more help, visit <a href="https://myapp.com/docs">our documentation</a>.</p>
`

widget := NewHTMLWidget(htmlContent)
```

### Handling Link Clicks

Link clicks are automatically detected and printed to stdout. To handle them in your application:

```go
// In your Update() loop, after widget.Update():
// Link clicks are printed as: "Clicked link: [URL]"
// You can modify the widget code to call a custom callback instead
```

## API Reference

### HTMLWidget

#### NewHTMLWidget(content string) *HTMLWidget
Creates a new HTML widget with the specified content.

#### Update()
Handles user input (scrolling, link interactions). Call once per frame.

#### Render(x, y, width, height float32)
Renders the HTML content within the specified bounds. Call once per frame after Update().

#### Unload()
Cleans up font resources. Call when the widget is no longer needed.

### HTMLElement

Represents a parsed HTML element with support for:
- Tag type and content
- Link URLs (`Href`)
- Heading levels (`Level`)
- Text formatting flags (`Bold`, `Italic`)
- Child elements for nested structures

## Font Support

MARQUEE automatically loads system fonts:

- **macOS**: Arial family from `/System/Library/Fonts/Supplemental/`
- **Windows**: Arial family from `C:/Windows/Fonts/`
- **Linux**: Liberation Sans from `/usr/share/fonts/truetype/liberation/`

Fonts are loaded at multiple sizes for headings (16px for body text, up to 32px for h1).

## Limitations

MARQUEE is intentionally minimal and does **not** support:

- CSS styling or external stylesheets
- JavaScript execution
- Images, videos, or multimedia content
- Complex layout (flexbox, grid, floats)
- Forms or input elements
- Nested formatting (e.g., bold italic text)
- Custom colors or themes (uses fixed color scheme)

These limitations keep the codebase small and focused on the core use case of documentation rendering.

## Technical Details

- **Parsing**: Uses regex-based HTML parsing optimized for the supported tag subset
- **Rendering**: Immediate-mode rendering recalculates layout each frame
- **Memory**: Minimal memory footprint, no DOM tree persistence
- **Performance**: Suitable for documents up to several pages; very large documents may impact framerate
- **Dependencies**: Only requires [raylib-go](https://github.com/gen2brain/raylib-go)

## Example HTML Structure

```html
<h1>Documentation Title</h1>
<p>This is a paragraph with <b>bold text</b> and <i>italic text</i>.</p>

<h2>Features</h2>
<ul>
    <li>Simple and <b>lightweight</b></li>
    <li>Cross-platform font support</li>
    <li>Clickable links: <a href="https://example.com">example.com</a></li>
</ul>

<hr>

<h3>Getting Started</h3>
<ol>
    <li>Install dependencies</li>
    <li>Copy the widget code</li>
    <li>Create your HTML content</li>
</ol>
```

## Running the Demo

```bash
# Compile and run with default example.html
go run marquee.go

# Or specify a custom HTML file
go run marquee.go my-documentation.html
```

).ReplaceAllString(html, "<hr>")
    
    // Convert lists (basic implementation)
    lines := strings.Split(html, "\n")
    var result []string
    inList := false
    
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        
        if strings.HasPrefix(trimmed, "- ") {
            if !inList {
                result = append(result, "<ul>")
                inList = true
            }
            content := strings.TrimPrefix(trimmed, "- ")
            result = append(result, "<li>"+content+"</li>")
        } else {
            if inList {
                result = append(result, "</ul>")
                inList = false
            }
            if trimmed != "" {
                result = append(result, "<p>"+trimmed+"</p>")
            }
        }
    }
    
    if inList {
        result = append(result, "</ul>")
    }
    
    return strings.Join(result, "\n")
}

func main() {
    rl.InitWindow(900, 700, "MARQUEE Markdown Viewer")
    defer rl.CloseWindow()

    // Load markdown file
    filename := "README.md"
    if len(os.Args) > 1 {
        filename = os.Args[1]
    }
    
    markdownBytes, err := os.ReadFile(filename)
    if err != nil {
        markdownBytes = []byte("# Error\nCould not load file: " + filename)
    }
    
    // Convert markdown to HTML
    htmlContent := markdownToHTML(string(markdownBytes))
    
    // Create MARQUEE widget
    widget := NewHTMLWidget(htmlContent)
    defer widget.Unload()

    rl.SetTargetFPS(60)
    
    for !rl.WindowShouldClose() {
        widget.Update()
        
        rl.BeginDrawing()
        rl.ClearBackground(rl.LightGray)
        widget.Render(20, 20, 800, 650)
        rl.EndDrawing()
    }
}
```

This creates a standalone Markdown viewer that converts common Markdown syntax to HTML on-the-fly and renders it using MARQUEE.

## Contact

- Email: h@ual.fi
- Mastodon: https://oldbytes.space/@haitchfive

## License

[Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0)
