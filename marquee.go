package marquee

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Global codepoints for Unicode support in TTF fonts
var essentialCodepoints []rune

func init() {
	// ASCII printable range (32-126)
	for i := rune(32); i <= 126; i++ {
		essentialCodepoints = append(essentialCodepoints, i)
	}

	// Essential Unicode punctuation
	essentialUnicode := []rune{
		0x2022, // • BULLET
		0x25CF, // ● BLACK CIRCLE
		0x2013, // – EN DASH
		0x2014, // — EM DASH
		0x201C, // " LEFT DOUBLE QUOTATION MARK
		0x201D, // " RIGHT DOUBLE QUOTATION MARK
	}
	essentialCodepoints = append(essentialCodepoints, essentialUnicode...)
}

// HTMLElement represents a parsed HTML element
type HTMLElement struct {
	Tag      string
	Content  string
	Href     string // For links
	Level    int    // For headings (h1=1, h2=2, etc.)
	Bold     bool   // For <b> tags
	Italic   bool   // For <i> tags
	Children []HTMLElement
}

// FontSet manages different font sizes for HTML elements
type FontSet struct {
	Regular    rl.Font
	Bold       rl.Font
	Italic     rl.Font
	BoldItalic rl.Font
	H1         rl.Font
	H2         rl.Font
	H3         rl.Font
	H4         rl.Font
	H5         rl.Font
	H6         rl.Font
}

// LinkArea represents a clickable region (for hyperlinks)
type LinkArea struct {
	Bounds rl.Rectangle
	URL    string
	Hover  bool
}

// HTMLWidget is the main widget for rendering HTML content
type HTMLWidget struct {
	Content        string
	Elements       []HTMLElement
	ScrollY        float32
	TargetScrollY  float32 // Target scroll position for smooth scrolling
	TotalHeight    float32
	WidgetHeight   float32 // Store the widget's render height
	Fonts          FontSet
	LinkAreas      []LinkArea
	ScrollbarAlpha float32 // For fade in/out effect
	LastScrollTime float32 // Time since last scroll for fade out
	// Body styling properties
	BodyMargin  float32
	BodyBorder  float32
	BodyPadding float32
	// Link click callback
	OnLinkClick func(string) // Callback for link clicks
}

// NewHTMLWidget creates a new HTML widget
func NewHTMLWidget(content string) *HTMLWidget {
	widget := &HTMLWidget{
		Content:        content,
		LinkAreas:      make([]LinkArea, 0),
		ScrollbarAlpha: 1.0, // Start visible, will fade if no interaction
		LastScrollTime: 0.0,
		// Default body styling (can be overridden by parsing <body> tags later)
		BodyMargin:  10.0,
		BodyBorder:  1.0,
		BodyPadding: 15.0,
	}

	// Load fonts
	widget.loadFonts()

	// Parse HTML content
	widget.Elements = widget.parseHTML(content)

	return widget
}

// Load TTF fonts at various sizes
func (w *HTMLWidget) loadFonts() {
	var fontPath, boldPath, italicPath string

	// Determine font paths based on OS
	if runtime.GOOS == "darwin" {
		fontPath = "/System/Library/Fonts/Supplemental/Arial.ttf"
		boldPath = "/System/Library/Fonts/Supplemental/Arial Bold.ttf"
		italicPath = "/System/Library/Fonts/Supplemental/Arial Italic.ttf"
	} else if runtime.GOOS == "windows" {
		fontPath = "C:/Windows/Fonts/arial.ttf"
		boldPath = "C:/Windows/Fonts/arialbd.ttf"
		italicPath = "C:/Windows/Fonts/ariali.ttf"
	} else {
		fontPath = "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf"
		boldPath = "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf"
		italicPath = "/usr/share/fonts/truetype/liberation/LiberationSans-Italic.ttf"
	}

	// Helper function to load font with fallback and Unicode support
	loadFontWithFallback := func(path string, size int32, fallback rl.Font) rl.Font {
		font := rl.LoadFontEx(path, size, essentialCodepoints)
		if font.BaseSize == 0 {
			return fallback
		}
		return font
	}

	// Load base regular font first with Unicode support
	// Create a minimal codepoints slice for essential Unicode characters
	var codepoints []rune

	// ASCII printable range (32-126)
	for i := rune(32); i <= 126; i++ {
		codepoints = append(codepoints, i)
	}

	// Essential Unicode punctuation
	essentialUnicode := []rune{
		0x2022, // • BULLET
		0x25CF, // ● BLACK CIRCLE
		0x2013, // – EN DASH
		0x2014, // — EM DASH
		0x201C, // " LEFT DOUBLE QUOTATION MARK
		0x201D, // " RIGHT DOUBLE QUOTATION MARK
	}
	codepoints = append(codepoints, essentialUnicode...)

	fmt.Printf("Loading font with %d codepoints including U+2022 and U+25CF\n", len(codepoints))

	regularFont := rl.LoadFontEx(fontPath, 16, codepoints)
	if regularFont.BaseSize == 0 {
		fmt.Printf("Failed to load TTF font, falling back to default\n")
		regularFont = rl.GetFontDefault()
	} else {
		fmt.Printf("Successfully loaded TTF font: %s\n", fontPath)
	}

	// Load other fonts with fallback to regular
	boldFont := loadFontWithFallback(boldPath, 16, regularFont)
	italicFont := loadFontWithFallback(italicPath, 16, regularFont)

	// Load heading fonts
	h1Font := loadFontWithFallback(fontPath, 32, regularFont)
	h2Font := loadFontWithFallback(fontPath, 28, regularFont)
	h3Font := loadFontWithFallback(fontPath, 24, regularFont)
	h4Font := loadFontWithFallback(fontPath, 20, regularFont)
	h5Font := loadFontWithFallback(fontPath, 18, regularFont)
	h6Font := loadFontWithFallback(fontPath, 16, regularFont)

	// Assign to FontSet struct
	w.Fonts = FontSet{
		Regular:    regularFont,
		Bold:       boldFont,
		Italic:     italicFont,
		BoldItalic: boldFont, // Use bold for now
		H1:         h1Font,
		H2:         h2Font,
		H3:         h3Font,
		H4:         h4Font,
		H5:         h5Font,
		H6:         h6Font,
	}
}

// Simple HTML parser based on version 1's approach with added formatting support
func (w *HTMLWidget) parseHTML(html string) []HTMLElement {
	var elements []HTMLElement
	html = strings.TrimSpace(html)

	// Regular expressions for parsing - includes bold and italic
	patterns := map[string]*regexp.Regexp{
		"heading": regexp.MustCompile(`(?s)<h([1-6])>(.*?)</h[1-6]>`),
		"link":    regexp.MustCompile(`(?s)<a\s+href="([^"]*)">(.*?)</a>`),
		"bold":    regexp.MustCompile(`(?s)<b>(.*?)</b>`),
		"italic":  regexp.MustCompile(`(?s)<i>(.*?)</i>`),
		"hr":      regexp.MustCompile(`(?s)<hr\s*/?>`),
		"ul":      regexp.MustCompile(`(?s)<ul>(.*?)</ul>`),
		"ol":      regexp.MustCompile(`(?s)<ol>(.*?)</ol>`),
		"p":       regexp.MustCompile(`(?s)<p>(.*?)</p>`),
		"br":      regexp.MustCompile(`(?s)<br\s*/?>`),
	}

	remaining := html

	for len(remaining) > 0 {
		found := false
		minIndex := len(remaining)
		var matchType string
		var matches []string

		// Find the earliest match across all patterns
		for patternType, pattern := range patterns {
			if pattern.MatchString(remaining) {
				loc := pattern.FindStringIndex(remaining)
				if loc != nil && loc[0] < minIndex {
					minIndex = loc[0]
					matchType = patternType
					matches = pattern.FindStringSubmatch(remaining)
					found = true
				}
			}
		}

		// Process text before the match
		if minIndex > 0 {
			text := strings.TrimSpace(remaining[:minIndex])
			if text != "" {
				elements = append(elements, HTMLElement{Tag: "text", Content: text})
			}
		}

		if found {
			element := w.createElementFromMatch(matchType, matches)
			if element.Tag != "" {
				elements = append(elements, element)
			}

			// Remove processed content - replace only first occurrence
			pattern := patterns[matchType]
			loc := pattern.FindStringIndex(remaining)
			if loc != nil {
				remaining = remaining[:loc[0]] + remaining[loc[1]:]
			}
		} else {
			// No more matches, add remaining text
			if strings.TrimSpace(remaining) != "" {
				elements = append(elements, HTMLElement{Tag: "text", Content: strings.TrimSpace(remaining)})
			}
			break
		}
	}

	return elements
}

// Create HTML element from regex match
func (w *HTMLWidget) createElementFromMatch(matchType string, matches []string) HTMLElement {
	switch matchType {
	case "heading":
		if len(matches) >= 3 {
			level, _ := strconv.Atoi(matches[1])
			return HTMLElement{Tag: "h" + matches[1], Content: matches[2], Level: level}
		}
	case "link":
		if len(matches) >= 3 {
			return HTMLElement{Tag: "a", Content: matches[2], Href: matches[1]}
		}
	case "bold":
		if len(matches) >= 2 {
			return HTMLElement{Tag: "b", Content: matches[1], Bold: true}
		}
	case "italic":
		if len(matches) >= 2 {
			return HTMLElement{Tag: "i", Content: matches[1], Italic: true}
		}
	case "hr":
		return HTMLElement{Tag: "hr"}
	case "ul", "ol":
		if len(matches) >= 2 {
			listItems := w.parseListItems(matches[1])
			return HTMLElement{Tag: matchType, Children: listItems}
		}
	case "p":
		if len(matches) >= 2 {
			return HTMLElement{Tag: "p", Content: matches[1]}
		}
	case "br":
		return HTMLElement{Tag: "br"}
	}
	return HTMLElement{}
}

// Parse list items from list content
func (w *HTMLWidget) parseListItems(content string) []HTMLElement {
	var items []HTMLElement
	re := regexp.MustCompile(`(?s)<li>(.*?)</li>`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			items = append(items, HTMLElement{Tag: "li", Content: match[1]})
		}
	}
	return items
}

// Parse text into segments with formatting preservation
func (w *HTMLWidget) parseTextSegments(text string) []HTMLElement {
	var segments []HTMLElement

	// If no HTML tags, return as simple text
	if !strings.Contains(text, "<") {
		return []HTMLElement{{Tag: "text", Content: text}}
	}

	// Use a sequential approach that preserves all whitespace
	pos := 0
	textLen := len(text)

	for pos < textLen {
		// Find the next tag
		nextTag := textLen
		tagType := ""

		// Check for each tag type
		if boldPos := strings.Index(text[pos:], "<b>"); boldPos >= 0 {
			if pos+boldPos < nextTag {
				nextTag = pos + boldPos
				tagType = "bold"
			}
		}
		if italicPos := strings.Index(text[pos:], "<i>"); italicPos >= 0 {
			if pos+italicPos < nextTag {
				nextTag = pos + italicPos
				tagType = "italic"
			}
		}
		if linkPos := strings.Index(text[pos:], "<a "); linkPos >= 0 {
			if pos+linkPos < nextTag {
				nextTag = pos + linkPos
				tagType = "link"
			}
		}

		// Add text before the tag (preserving whitespace!)
		if nextTag > pos {
			beforeText := text[pos:nextTag]
			if beforeText != "" {
				segments = append(segments, HTMLElement{Tag: "text", Content: beforeText})
			}
		}

		// Process the tag
		if tagType == "" {
			// No more tags
			break
		}

		switch tagType {
		case "bold":
			if endPos := strings.Index(text[nextTag:], "</b>"); endPos >= 0 {
				content := text[nextTag+3 : nextTag+endPos]
				segments = append(segments, HTMLElement{Tag: "b", Content: content, Bold: true})
				pos = nextTag + endPos + 4 // Skip past </b>
			} else {
				pos = nextTag + 3 // Skip past <b>
			}
		case "italic":
			if endPos := strings.Index(text[nextTag:], "</i>"); endPos >= 0 {
				content := text[nextTag+3 : nextTag+endPos]
				segments = append(segments, HTMLElement{Tag: "i", Content: content, Italic: true})
				pos = nextTag + endPos + 4 // Skip past </i>
			} else {
				pos = nextTag + 3 // Skip past <i>
			}
		case "link":
			if hrefEnd := strings.Index(text[nextTag:], "\">"); hrefEnd >= 0 {
				if linkEnd := strings.Index(text[nextTag+hrefEnd:], "</a>"); linkEnd >= 0 {
					href := text[nextTag+9 : nextTag+hrefEnd] // Skip '<a href="'
					content := text[nextTag+hrefEnd+2 : nextTag+hrefEnd+linkEnd]
					segments = append(segments, HTMLElement{Tag: "a", Content: content, Href: href})
					pos = nextTag + hrefEnd + linkEnd + 4 // Skip past </a>
				} else {
					pos = nextTag + 3 // Skip malformed tag
				}
			} else {
				pos = nextTag + 3 // Skip malformed tag
			}
		}
	}

	// Add any remaining text
	if pos < textLen {
		remaining := text[pos:]
		if remaining != "" {
			segments = append(segments, HTMLElement{Tag: "text", Content: remaining})
		}
	}

	return segments
}

// Update widget state - fixed scroll limiting and scrollbar sync
func (w *HTMLWidget) Update() {
	// Reset mouse cursor at start of each update
	rl.SetMouseCursor(rl.MouseCursorDefault)

	// Handle scrolling
	wheel := rl.GetMouseWheelMove()
	w.ScrollY -= wheel * 20

	// FIXED: Clamp scroll position to valid range
	if w.ScrollY < 0 {
		w.ScrollY = 0
	}

	// Calculate maximum scroll position based on content vs widget height
	// Use the widget render height (typically 650 from the main function)
	widgetHeight := float32(650) // This should match the height passed to Render()
	maxScroll := w.TotalHeight - widgetHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	if w.ScrollY > maxScroll {
		w.ScrollY = maxScroll
	}

	// Update link hover states and handle clicks
	mousePos := rl.GetMousePosition()
	for i := range w.LinkAreas {
		area := &w.LinkAreas[i]

		// Convert document coordinates to screen coordinates for collision detection
		screenBounds := area.Bounds
		screenBounds.Y -= w.ScrollY

		area.Hover = rl.CheckCollisionPointRec(mousePos, screenBounds)

		if area.Hover {
			rl.SetMouseCursor(rl.MouseCursorPointingHand)
		}

		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && area.Hover {
			if w.OnLinkClick != nil {
				w.OnLinkClick(area.URL)
			} else {
				fmt.Printf("Clicked link: %s\n", area.URL)
			}
		}
	}
}

// Render the HTML widget with stable content height calculation
func (w *HTMLWidget) Render(x, y, width, height float32) {
	w.LinkAreas = w.LinkAreas[:0] // Clear previous link areas

	// Store widget height for scroll calculations
	w.WidgetHeight = height

	// Draw white background for the widget area
	rl.DrawRectangle(int32(x), int32(y), int32(width), int32(height), rl.White)

	// Draw border around widget area
	if w.BodyBorder > 0 {
		borderColor := rl.Color{R: 200, G: 200, B: 200, A: 255} // Light border
		rl.DrawRectangleLinesEx(
			rl.NewRectangle(x, y, width, height),
			w.BodyBorder,
			borderColor)
	}

	// Calculate content area - margin and padding are part of the PAGE, not widget chrome
	contentX := x + w.BodyMargin + w.BodyPadding
	contentY := y + w.BodyMargin + w.BodyPadding - w.ScrollY
	contentWidth := width - 2*(w.BodyMargin+w.BodyPadding)

	// Enable clipping for scrolling - clip to FULL widget area, not reduced by margin
	rl.BeginScissorMode(
		int32(x),
		int32(y),
		int32(width),
		int32(height))

	currentY := contentY
	for _, element := range w.Elements {
		currentY = w.renderElement(element, contentX, currentY, contentWidth, 0)
	}

	// FIXED: Only update TotalHeight once, keep it stable
	if w.TotalHeight == 0 {
		w.TotalHeight = currentY + w.ScrollY - contentY + 2*(w.BodyMargin+w.BodyPadding)
	}

	rl.EndScissorMode()

	// Draw fading scrollbar if needed
	if w.TotalHeight > height {
		w.drawScrollbar(x, y, width, height)
	}
}

// Render individual HTML elements
func (w *HTMLWidget) renderElement(element HTMLElement, x, y, width float32, indent int) float32 {
	switch element.Tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return w.renderHeading(element, x, y, width)
	case "p":
		return w.renderFormattedText(element.Content, x, y, width)
	case "text":
		return w.renderText(element.Content, x, y, width, w.Fonts.Regular, rl.Black)
	case "b":
		return w.renderText(element.Content, x, y, width, w.Fonts.Bold, rl.DarkBlue)
	case "i":
		return w.renderText(element.Content, x, y, width, w.Fonts.Italic, rl.DarkGreen)
	case "a":
		return w.renderLink(element, x, y, width)
	case "hr":
		return w.renderHR(x, y, width)
	case "ul", "ol":
		return w.renderList(element, x, y, width, indent)
	case "br":
		return y + 20
	default:
		return y
	}
}

// Render heading with appropriate font size and Unicode support
func (w *HTMLWidget) renderHeading(element HTMLElement, x, y, width float32) float32 {
	var font rl.Font

	switch element.Tag {
	case "h1":
		font = w.Fonts.H1
	case "h2":
		font = w.Fonts.H2
	case "h3":
		font = w.Fonts.H3
	case "h4":
		font = w.Fonts.H4
	case "h5":
		font = w.Fonts.H5
	case "h6":
		font = w.Fonts.H6
	default:
		font = w.Fonts.Regular
	}

	// Add spacing before heading
	y += 15

	fontSize := float32(font.BaseSize)
	if font.BaseSize == 0 { // Fallback font
		fontSize = float32([]int{40, 30, 20, 20, 10, 10}[element.Level-1])
	}

	// Use Unicode-aware rendering for headings
	w.renderTextWithUnicode(element.Content, x, y, font, rl.DarkBlue)

	return y + fontSize + 10
}

// Render regular text with word wrapping and Unicode support
func (w *HTMLWidget) renderText(text string, x, y, width float32, font rl.Font, color rl.Color) float32 {
	if text == "" {
		return y
	}

	words := strings.Fields(text)
	currentLine := ""
	lineHeight := float32(20)
	currentY := y

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		textWidth := rl.MeasureTextEx(font, testLine, float32(font.BaseSize), 1).X
		if textWidth > width-40 && currentLine != "" {
			// Draw current line with Unicode support
			w.renderTextWithUnicode(currentLine, x, currentY, font, color)
			currentY += lineHeight
			currentLine = word
		} else {
			currentLine = testLine
		}
	}

	// Draw remaining text with Unicode support
	if currentLine != "" {
		w.renderTextWithUnicode(currentLine, x, currentY, font, color)
		currentY += lineHeight
	}

	return currentY + 5
}

// Helper function to render text with proper Unicode handling
func (w *HTMLWidget) renderTextWithUnicode(text string, x, y float32, font rl.Font, color rl.Color) {
	currentX := x
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16 // fallback
	}

	// Convert string to runes for proper Unicode handling
	runes := []rune(text)

	for _, r := range runes {
		if r < 128 {
			// ASCII character - use DrawTextEx for efficiency
			charStr := string(r)
			charWidth := rl.MeasureTextEx(font, charStr, fontSize, 1).X
			rl.DrawTextEx(font, charStr, rl.NewVector2(currentX, y), fontSize, 1, color)
			currentX += charWidth
		} else {
			// Unicode character - use DrawTextCodepoint
			rl.DrawTextCodepoint(font, r, rl.NewVector2(currentX, y), fontSize, color)
			// Better character width calculation for Unicode
			charWidth := rl.MeasureTextEx(font, "M", fontSize, 1).X * 0.8 // Use M-width as baseline
			currentX += charWidth
		}
	}
}

// Render formatted text with inline elements - preserves exact spacing from original text
func (w *HTMLWidget) renderFormattedText(text string, x, y, width float32) float32 {
	if text == "" {
		return y
	}

	// Check if text contains any HTML tags to format
	if !strings.Contains(text, "<") {
		// No formatting needed, use simple text rendering
		return w.renderText(text, x, y, width, w.Fonts.Regular, rl.Black)
	}

	// Parse inline formatting
	segments := w.parseTextSegments(text)

	if len(segments) == 0 {
		// Fallback to simple text if parsing failed
		return w.renderText(text, x, y, width, w.Fonts.Regular, rl.Black)
	}

	// Render all segments as one continuous flow, preserving original spacing
	currentX := x
	currentY := y
	lineHeight := float32(20)

	for _, segment := range segments {
		var font rl.Font
		var color rl.Color

		switch segment.Tag {
		case "b":
			font = w.Fonts.Bold
			color = rl.DarkBlue
		case "i":
			font = w.Fonts.Italic
			color = rl.DarkGreen
		case "a":
			font = w.Fonts.Regular
			color = rl.Blue
		default:
			font = w.Fonts.Regular
			color = rl.Black
		}

		// Handle links specially for clickable areas
		if segment.Tag == "a" {
			fontSize := float32(font.BaseSize)
			if fontSize == 0 {
				fontSize = 16
			}

			textSize := rl.MeasureTextEx(font, segment.Content, fontSize, 1)

			// Check if link fits on current line
			if currentX+textSize.X > x+width-40 && currentX > x {
				currentY += lineHeight
				currentX = x
			}

			// Store link area in document coordinates
			documentY := currentY + w.ScrollY
			bounds := rl.NewRectangle(currentX, documentY, textSize.X, textSize.Y)
			linkArea := LinkArea{Bounds: bounds, URL: segment.Href}
			w.LinkAreas = append(w.LinkAreas, linkArea)

			// Render link
			w.renderTextWithUnicode(segment.Content, currentX, currentY, font, color)
			textSize = rl.MeasureTextEx(font, segment.Content, fontSize, 1)
			rl.DrawLineEx(
				rl.NewVector2(currentX, currentY+textSize.Y),
				rl.NewVector2(currentX+textSize.X, currentY+textSize.Y),
				1, color)

			currentX += textSize.X
			continue
		}

		// Handle text segments word-by-word BUT preserve original spacing
		if segment.Tag == "text" {
			// FIXED: Don't use Fields() - instead split manually to preserve spaces
			content := segment.Content
			currentPos := 0

			for currentPos < len(content) {
				// Find next word boundary (space or end)
				wordStart := currentPos
				for currentPos < len(content) && content[currentPos] != ' ' {
					currentPos++
				}

				// Extract word
				word := content[wordStart:currentPos]

				// Count following spaces
				spaceCount := 0
				for currentPos < len(content) && content[currentPos] == ' ' {
					currentPos++
					spaceCount++
				}

				// Add the spaces to the word
				word += strings.Repeat(" ", spaceCount)

				if word == "" {
					continue
				}

				fontSize := float32(font.BaseSize)
				if fontSize == 0 {
					fontSize = 16
				}

				wordWidth := rl.MeasureTextEx(font, word, fontSize, 1).X

				// Check if word fits on current line
				if currentX+wordWidth > x+width-40 && currentX > x {
					currentY += lineHeight
					currentX = x
					// Remove leading spaces if wrapping to new line
					word = strings.TrimLeft(word, " ")
					wordWidth = rl.MeasureTextEx(font, word, fontSize, 1).X
				}

				// Render word with preserved spacing
				if word != "" {
					w.renderTextWithUnicode(word, currentX, currentY, font, color)
					wordWidth = rl.MeasureTextEx(font, word, fontSize, 1).X // Re-measure for positioning
					currentX += wordWidth
				}
			}
		} else {
			// Handle bold/italic segments (already processed content)
			fontSize := float32(font.BaseSize)
			if fontSize == 0 {
				fontSize = 16
			}

			textWidth := rl.MeasureTextEx(font, segment.Content, fontSize, 1).X

			// Check if segment fits on current line
			if currentX+textWidth > x+width-40 && currentX > x {
				currentY += lineHeight
				currentX = x
			}

			// Render formatted text
			w.renderTextWithUnicode(segment.Content, currentX, currentY, font, color)
			currentX += textWidth
		}
	}

	return currentY + lineHeight + 5
}

// Render clickable hyperlinks with Unicode support
func (w *HTMLWidget) renderLink(element HTMLElement, x, y, width float32) float32 {
	font := w.Fonts.Regular
	fontSize := float32(font.BaseSize)

	textSize := rl.MeasureTextEx(font, element.Content, fontSize, 1)

	// Create clickable area in document coordinates
	documentY := y + w.ScrollY // Convert screen Y back to document Y
	bounds := rl.NewRectangle(x, documentY, textSize.X, textSize.Y)
	linkArea := LinkArea{Bounds: bounds, URL: element.Href}
	w.LinkAreas = append(w.LinkAreas, linkArea)

	// Determine color based on hover state
	color := rl.Blue
	for _, area := range w.LinkAreas {
		if area.URL == element.Href && area.Hover {
			color = rl.DarkBlue
		}
	}

	// Render link text with Unicode support
	w.renderTextWithUnicode(element.Content, x, y, font, color)

	// Draw underline
	rl.DrawLineEx(
		rl.NewVector2(x, y+textSize.Y),
		rl.NewVector2(x+textSize.X, y+textSize.Y),
		1, color)

	return y + textSize.Y + 5
}

// Render horizontal rule
func (w *HTMLWidget) renderHR(x, y, width float32) float32 {
	y += 10
	rl.DrawLineEx(
		rl.NewVector2(x, y),
		rl.NewVector2(x+width-40, y),
		2, rl.Gray)
	return y + 15
}

// Render lists (ul/ol)
func (w *HTMLWidget) renderList(element HTMLElement, x, y, width float32, indent int) float32 {
	y += 10
	indentX := x + float32(indent*20)

	for i, item := range element.Children {
		if item.Tag == "li" {
			// Draw bullet or number
			if element.Tag == "ol" {
				marker := fmt.Sprintf("%d.", i+1)
				rl.DrawTextEx(w.Fonts.Regular, marker, rl.NewVector2(indentX, y), 16, 1, rl.Black)
			} else {
				// For bullets, ensure proper UTF-8 handling by explicitly using the Unicode codepoint
				bulletRune := rune(0x2022) // • BULLET
				bulletStr := string(bulletRune)
				rl.DrawTextEx(w.Fonts.Regular, bulletStr, rl.NewVector2(indentX, y), 16, 1, rl.Black)
			}

			// Render list item content using formatted text rendering
			y = w.renderFormattedText(item.Content, indentX+25, y, width-25-float32(indent*20))
		}
	}

	return y + 5
}

// Draw clean, stable scrollbar
func (w *HTMLWidget) drawScrollbar(x, y, width, height float32) {
	if w.TotalHeight <= height || w.ScrollbarAlpha <= 0.01 {
		return
	}

	scrollbarWidth := float32(10)
	scrollbarX := x + width - scrollbarWidth

	// FIXED: Constant thumb size based on reasonable proportion
	// Use a fixed thumb size that represents a reasonable viewing window
	contentArea := height - 2*w.BodyMargin
	thumbHeight := contentArea * 0.2 // Always 20% of available space

	// Ensure reasonable bounds
	if thumbHeight < 40 {
		thumbHeight = 40
	}
	if thumbHeight > contentArea*0.8 {
		thumbHeight = contentArea * 0.8
	}

	// Calculate scroll progress for positioning
	maxScroll := w.TotalHeight - height
	if maxScroll <= 0 {
		return // No scrolling needed
	}

	scrollProgress := w.ScrollY / maxScroll
	if scrollProgress < 0 {
		scrollProgress = 0
	}
	if scrollProgress > 1 {
		scrollProgress = 1
	}

	// Position thumb within available track space
	trackHeight := contentArea - thumbHeight
	thumbY := y + w.BodyMargin + scrollProgress*trackHeight

	// Create color with fade alpha
	alpha := uint8(w.ScrollbarAlpha * 120)
	thumbColor := rl.Color{R: 60, G: 60, B: 60, A: alpha}

	// Draw the stable-sized thumb
	rl.DrawRectangle(
		int32(scrollbarX),
		int32(thumbY),
		int32(scrollbarWidth),
		int32(thumbHeight),
		thumbColor)
}

// Fixed font cleanup to prevent double free crashes
func (w *HTMLWidget) Unload() {
	defaultFont := rl.GetFontDefault()

	// Keep track of already unloaded fonts to prevent double-free
	unloadedIDs := make(map[uint32]bool)

	// Helper function to safely unload fonts
	safeUnload := func(font rl.Font, name string) {
		if font.BaseSize > 0 && font.BaseSize != defaultFont.BaseSize {
			// Check if this texture ID was already unloaded
			if !unloadedIDs[font.Texture.ID] && font.Texture.ID != defaultFont.Texture.ID {
				rl.UnloadFont(font)
				unloadedIDs[font.Texture.ID] = true
				fmt.Printf("Unloaded font: %s (ID: %d)\n", name, font.Texture.ID)
			}
		}
	}

	// Unload all fonts safely, avoiding duplicates
	safeUnload(w.Fonts.Regular, "Regular")
	safeUnload(w.Fonts.Bold, "Bold")
	safeUnload(w.Fonts.Italic, "Italic")
	safeUnload(w.Fonts.BoldItalic, "BoldItalic")
	safeUnload(w.Fonts.H1, "H1")
	safeUnload(w.Fonts.H2, "H2")
	safeUnload(w.Fonts.H3, "H3")
	safeUnload(w.Fonts.H4, "H4")
	safeUnload(w.Fonts.H5, "H5")
	safeUnload(w.Fonts.H6, "H6")
}
