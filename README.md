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

## Running the Demo

```bash
# HTML Viewer - run from the htmlview directory
cd examples/htmlview
go run htmlview.go example.html

# Or view other sample files
go run htmlview.go blog.html
go run htmlview.go menu.html

# MarqueeDown - run from the marqueedown directory to view linked documentation
cd examples/marqueedown
go run marqueedown.go

# Or specify a custom Markdown file
go run marqueedown.go index.md
```

Both examples come with sample documentation files that demonstrate MARQUEE's features. The htmlview example includes several HTML files (blog.html, example.html, menu.html), while the marqueedown example contains interconnected markdown documentation files with working hyperlinks between documents.

## Contact

- Email: h@ual.fi
- Mastodon: https://oldbytes.space/@haitchfive

## License

[Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0)
