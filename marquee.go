package marquee

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

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

// GlobalFontManager manages shared font resources across all widgets
type GlobalFontManager struct {
	fonts         map[string]rl.Font // key = "fontname:size"
	refCounts     map[string]int     // reference counting for cleanup
	mutex         sync.RWMutex
	fontPaths     map[string]string // platform-specific font paths
	monoFontPaths map[string]string // platform-specific monospace font paths
	initialized   bool
}

// Global singleton font manager instance
var fontManager *GlobalFontManager
var fontManagerOnce sync.Once

// getFontManager returns the singleton font manager instance
func getFontManager() *GlobalFontManager {
	fontManagerOnce.Do(func() {
		fontManager = &GlobalFontManager{
			fonts:         make(map[string]rl.Font),
			refCounts:     make(map[string]int),
			fontPaths:     make(map[string]string),
			monoFontPaths: make(map[string]string),
		}
		fontManager.initializePlatformPaths()
	})
	return fontManager
}

// initializePlatformPaths sets up font paths based on operating system
func (fm *GlobalFontManager) initializePlatformPaths() {
	if runtime.GOOS == "darwin" {
		// Regular fonts
		fm.fontPaths["arial"] = "/System/Library/Fonts/Supplemental/Arial.ttf"
		fm.fontPaths["arial-bold"] = "/System/Library/Fonts/Supplemental/Arial Bold.ttf"
		fm.fontPaths["arial-italic"] = "/System/Library/Fonts/Supplemental/Arial Italic.ttf"
		// Monospace fonts (in order of preference)
		fm.monoFontPaths["monaco"] = "/System/Library/Fonts/Monaco.ttf"
		fm.monoFontPaths["menlo"] = "/System/Library/Fonts/Menlo.ttc"
		fm.monoFontPaths["courier"] = "/System/Library/Fonts/Courier.ttc"
	} else if runtime.GOOS == "windows" {
		// Regular fonts
		fm.fontPaths["arial"] = "C:/Windows/Fonts/arial.ttf"
		fm.fontPaths["arial-bold"] = "C:/Windows/Fonts/arialbd.ttf"
		fm.fontPaths["arial-italic"] = "C:/Windows/Fonts/ariali.ttf"
		// Monospace fonts (in order of preference)
		fm.monoFontPaths["consolas"] = "C:/Windows/Fonts/consola.ttf"
		fm.monoFontPaths["cascadia"] = "C:/Windows/Fonts/CascadiaCode.ttf"
		fm.monoFontPaths["courier"] = "C:/Windows/Fonts/cour.ttf"
		fm.monoFontPaths["lucida-console"] = "C:/Windows/Fonts/lucon.ttf"
	} else {
		// Regular fonts
		fm.fontPaths["arial"] = "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf"
		fm.fontPaths["arial-bold"] = "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf"
		fm.fontPaths["arial-italic"] = "/usr/share/fonts/truetype/liberation/LiberationSans-Italic.ttf"
		// Monospace fonts (in order of preference)
		fm.monoFontPaths["dejavu-mono"] = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
		fm.monoFontPaths["liberation-mono"] = "/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf"
		fm.monoFontPaths["ubuntu-mono"] = "/usr/share/fonts/truetype/ubuntu/UbuntuMono-R.ttf"
		fm.monoFontPaths["courier"] = "/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf"
	}
	fm.initialized = true
}

// GetMonospaceFont returns the best available monospace font for the platform
func (fm *GlobalFontManager) GetMonospaceFont(size int32) rl.Font {
	// Try to get an existing monospace font
	key := fmt.Sprintf("monospace:%d", size)

	fm.mutex.RLock()
	if font, exists := fm.fonts[key]; exists {
		fm.mutex.RUnlock()
		fm.mutex.Lock()
		fm.refCounts[key]++
		fm.mutex.Unlock()
		return font
	}
	fm.mutex.RUnlock()

	// Need to detect and load a monospace font
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	// Double-check after acquiring write lock
	if font, exists := fm.fonts[key]; exists {
		fm.refCounts[key]++
		return font
	}

	// Try fonts in order of preference for this platform
	var fontOrder []string
	if runtime.GOOS == "darwin" {
		fontOrder = []string{"monaco", "menlo", "sf-mono", "courier"}
	} else if runtime.GOOS == "windows" {
		fontOrder = []string{"consolas", "cascadia", "courier", "lucida-console"}
	} else {
		fontOrder = []string{"dejavu-mono", "liberation-mono", "ubuntu-mono", "courier"}
	}

	var loadedFont rl.Font

	// Try each font until one loads successfully
	for _, fontName := range fontOrder {
		if fontPath, exists := fm.monoFontPaths[fontName]; exists {
			testFont := rl.LoadFontEx(fontPath, size, essentialCodepoints)
			if testFont.BaseSize > 0 {
				loadedFont = testFont
				fmt.Printf("Loaded monospace font: %s at size %d\n", fontName, size)
				break
			}
		}
	}

	// If no monospace font loaded, fall back to default font
	if loadedFont.BaseSize == 0 {
		loadedFont = rl.GetFontDefault()
		fmt.Printf("Using default font as monospace fallback\n")
	}

	fm.fonts[key] = loadedFont
	fm.refCounts[key] = 1

	return loadedFont
}

// GetFont returns a shared font, loading it if necessary
func (fm *GlobalFontManager) GetFont(fontName string, size int32) rl.Font {
	key := fmt.Sprintf("%s:%d", fontName, size)

	// Check if font already exists
	fm.mutex.RLock()
	if font, exists := fm.fonts[key]; exists {
		fm.mutex.RUnlock()
		// Increment reference count
		fm.mutex.Lock()
		fm.refCounts[key]++
		fm.mutex.Unlock()
		return font
	}
	fm.mutex.RUnlock()

	// Font doesn't exist, need to load it
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	// Double-check after acquiring write lock
	if font, exists := fm.fonts[key]; exists {
		fm.refCounts[key]++
		return font
	}

	// Load the font
	fontPath, pathExists := fm.fontPaths[fontName]
	if !pathExists {
		// Fallback to default font
		defaultFont := rl.GetFontDefault()
		fm.fonts[key] = defaultFont
		fm.refCounts[key] = 1
		return defaultFont
	}

	font := rl.LoadFontEx(fontPath, size, essentialCodepoints)
	if font.BaseSize == 0 {
		// Failed to load, use default
		font = rl.GetFontDefault()
	}

	fm.fonts[key] = font
	fm.refCounts[key] = 1

	if font.BaseSize > 0 {
		fmt.Printf("Loaded shared font: %s at size %d (key: %s)\n", fontName, size, key)
	}

	return font
}

// ReleaseFont decrements reference count and unloads if no longer used
func (fm *GlobalFontManager) ReleaseFont(fontName string, size int32) {
	key := fmt.Sprintf("%s:%d", fontName, size)

	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if count, exists := fm.refCounts[key]; exists {
		count--
		if count <= 0 {
			// No more references, safe to unload
			if font, fontExists := fm.fonts[key]; fontExists {
				defaultFont := rl.GetFontDefault()
				// Only unload if it's not the default font
				if font.BaseSize > 0 && font.Texture.ID != defaultFont.Texture.ID {
					rl.UnloadFont(font)
					fmt.Printf("Unloaded shared font: %s (key: %s)\n", fontName, key)
				}
			}
			delete(fm.fonts, key)
			delete(fm.refCounts, key)
		} else {
			fm.refCounts[key] = count
		}
	}
}

// ReleaseMonospaceFont releases monospace font references
func (fm *GlobalFontManager) ReleaseMonospaceFont(size int32) {
	key := fmt.Sprintf("monospace:%d", size)

	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if count, exists := fm.refCounts[key]; exists {
		count--
		if count <= 0 {
			if font, fontExists := fm.fonts[key]; fontExists {
				defaultFont := rl.GetFontDefault()
				if font.BaseSize > 0 && font.Texture.ID != defaultFont.Texture.ID {
					rl.UnloadFont(font)
					fmt.Printf("Unloaded monospace font (key: %s)\n", key)
				}
			}
			delete(fm.fonts, key)
			delete(fm.refCounts, key)
		} else {
			fm.refCounts[key] = count
		}
	}
}

// TextMeasureCache provides fast text measurement with LRU caching
type TextMeasureCache struct {
	cache       map[string]rl.Vector2
	accessOrder []string // For LRU tracking
	maxEntries  int
}

// NewTextMeasureCache creates a new text measurement cache
func NewTextMeasureCache(maxEntries int) *TextMeasureCache {
	return &TextMeasureCache{
		cache:       make(map[string]rl.Vector2),
		accessOrder: make([]string, 0),
		maxEntries:  maxEntries,
	}
}

// GetTextSize returns cached text measurements or calculates and caches new ones
func (tmc *TextMeasureCache) GetTextSize(font rl.Font, text string, fontSize float32) rl.Vector2 {
	// Create cache key from font texture ID, size, and text
	key := fmt.Sprintf("%d:%.1f:%s", font.Texture.ID, fontSize, text)

	// Check if measurement exists in cache
	if size, exists := tmc.cache[key]; exists {
		// Move to end of access order for LRU
		tmc.updateAccessOrder(key)
		return size
	}

	// Calculate measurement
	size := rl.MeasureTextEx(font, text, fontSize, 1)

	// Add to cache
	tmc.cache[key] = size
	tmc.accessOrder = append(tmc.accessOrder, key)

	// Enforce LRU limit
	if len(tmc.cache) > tmc.maxEntries {
		// Remove oldest entry
		oldestKey := tmc.accessOrder[0]
		delete(tmc.cache, oldestKey)
		tmc.accessOrder = tmc.accessOrder[1:]
	}

	return size
}

// GetTextWidth returns just the width component for convenience
func (tmc *TextMeasureCache) GetTextWidth(font rl.Font, text string, fontSize float32) float32 {
	return tmc.GetTextSize(font, text, fontSize).X
}

// updateAccessOrder moves a key to the end for LRU tracking
func (tmc *TextMeasureCache) updateAccessOrder(key string) {
	// Find and remove the key from current position
	for i, k := range tmc.accessOrder {
		if k == key {
			// Remove from current position
			tmc.accessOrder = append(tmc.accessOrder[:i], tmc.accessOrder[i+1:]...)
			break
		}
	}
	// Add to end
	tmc.accessOrder = append(tmc.accessOrder, key)
}

// Clear empties the cache (useful for testing or memory pressure)
func (tmc *TextMeasureCache) Clear() {
	tmc.cache = make(map[string]rl.Vector2)
	tmc.accessOrder = tmc.accessOrder[:0]
}

// ElementHandler defines the interface for handling different HTML element types
type ElementHandler interface {
	// GetPattern returns the regex pattern for finding this element type
	GetPattern() *regexp.Regexp
	// ParseMatched extracts this element from the matched content
	ParseMatched(matches []string) HTMLElement
	// Render draws the element and returns the next Y position
	Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32
}

// ElementIndex manages the mapping of HTML tags to their handlers
type ElementIndex struct {
	handlers map[string]ElementHandler
}

// NewElementIndex creates a new element index with default handlers
func NewElementIndex() *ElementIndex {
	index := &ElementIndex{
		handlers: make(map[string]ElementHandler),
	}

	// Register handlers for each specific tag
	headingHandler := &HeadingHandler{}
	index.RegisterHandler("h1", headingHandler)
	index.RegisterHandler("h2", headingHandler)
	index.RegisterHandler("h3", headingHandler)
	index.RegisterHandler("h4", headingHandler)
	index.RegisterHandler("h5", headingHandler)
	index.RegisterHandler("h6", headingHandler)

	index.RegisterHandler("a", &LinkHandler{})
	index.RegisterHandler("b", &BoldHandler{})
	index.RegisterHandler("i", &ItalicHandler{})
	index.RegisterHandler("hr", &HRHandler{})
	index.RegisterHandler("ul", &ListHandler{})
	index.RegisterHandler("ol", &ListHandler{})
	index.RegisterHandler("p", &ParagraphHandler{})
	index.RegisterHandler("br", &BreakHandler{})

	// NEW: Register code handlers
	index.RegisterHandler("pre", &PreHandler{})
	index.RegisterHandler("code", &CodeHandler{})

	return index
}

// RegisterHandler adds or updates a handler for an element type
func (ei *ElementIndex) RegisterHandler(elementType string, handler ElementHandler) {
	ei.handlers[elementType] = handler
}

// GetHandler returns the handler for an element type, or nil if not found
func (ei *ElementIndex) GetHandler(elementType string) ElementHandler {
	return ei.handlers[elementType]
}

// FindFirstElement finds the earliest HTML element in content using all registered handlers
func (ei *ElementIndex) FindFirstElement(content string) (string, HTMLElement, string, bool) {
	minIndex := len(content)
	var bestElementType string
	var bestElement HTMLElement
	var bestRemaining string

	// Try each registered handler
	for elementType, handler := range ei.handlers {
		pattern := handler.GetPattern()
		if matches := pattern.FindStringSubmatch(content); matches != nil {
			// Find where this match starts
			loc := pattern.FindStringIndex(content)
			if loc != nil && loc[0] < minIndex {
				minIndex = loc[0]
				bestElementType = elementType
				bestElement = handler.ParseMatched(matches)
				// Calculate remaining content after this match
				bestRemaining = content[:loc[0]] + content[loc[1]:]
			}
		}
	}

	if bestElementType != "" {
		return bestElementType, bestElement, bestRemaining, true
	}

	return "", HTMLElement{}, content, false
}

// NEW: PreHandler handles <pre> elements (preformatted text blocks)
type PreHandler struct{}

func (p *PreHandler) GetPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<pre>(.*?)</pre>`)
}

func (p *PreHandler) ParseMatched(matches []string) HTMLElement {
	if len(matches) >= 2 {
		return HTMLElement{Tag: "pre", Content: matches[1]}
	}
	return HTMLElement{}
}

func (p *PreHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return widget.renderPreBlock(element, x, y, width)
}

// NEW: CodeHandler handles <code> elements (inline code)
type CodeHandler struct{}

func (c *CodeHandler) GetPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<code>(.*?)</code>`)
}

func (c *CodeHandler) ParseMatched(matches []string) HTMLElement {
	if len(matches) >= 2 {
		return HTMLElement{Tag: "code", Content: matches[1]}
	}
	return HTMLElement{}
}

func (c *CodeHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return widget.renderInlineCode(element, x, y, width)
}

// Handler implementations for existing HTML elements

// HeadingHandler handles h1-h6 elements
type HeadingHandler struct{}

func (h *HeadingHandler) GetPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<h([1-6])>(.*?)</h[1-6]>`)
}

func (h *HeadingHandler) ParseMatched(matches []string) HTMLElement {
	if len(matches) >= 3 {
		level, _ := strconv.Atoi(matches[1])
		return HTMLElement{Tag: "h" + matches[1], Content: matches[2], Level: level}
	}
	return HTMLElement{}
}

func (h *HeadingHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return widget.renderHeading(element, x, y, width)
}

// LinkHandler handles <a> elements
type LinkHandler struct{}

func (l *LinkHandler) GetPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<a\s+href="([^"]*)">(.*?)</a>`)
}

func (l *LinkHandler) ParseMatched(matches []string) HTMLElement {
	if len(matches) >= 3 {
		return HTMLElement{Tag: "a", Content: matches[2], Href: matches[1]}
	}
	return HTMLElement{}
}

func (l *LinkHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return widget.renderLink(element, x, y, width)
}

// BoldHandler handles <b> elements
type BoldHandler struct{}

func (b *BoldHandler) GetPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<b>(.*?)</b>`)
}

func (b *BoldHandler) ParseMatched(matches []string) HTMLElement {
	if len(matches) >= 2 {
		return HTMLElement{Tag: "b", Content: matches[1], Bold: true}
	}
	return HTMLElement{}
}

func (b *BoldHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return widget.renderText(element.Content, x, y, width, widget.Fonts.Bold, rl.DarkBlue)
}

// ItalicHandler handles <i> elements
type ItalicHandler struct{}

func (i *ItalicHandler) GetPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<i>(.*?)</i>`)
}

func (i *ItalicHandler) ParseMatched(matches []string) HTMLElement {
	if len(matches) >= 2 {
		return HTMLElement{Tag: "i", Content: matches[1], Italic: true}
	}
	return HTMLElement{}
}

func (i *ItalicHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return widget.renderText(element.Content, x, y, width, widget.Fonts.Italic, rl.DarkGreen)
}

// HRHandler handles <hr> elements
type HRHandler struct{}

func (h *HRHandler) GetPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<hr\s*/?>`)
}

func (h *HRHandler) ParseMatched(matches []string) HTMLElement {
	return HTMLElement{Tag: "hr"}
}

func (h *HRHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return widget.renderHR(x, y, width)
}

// ListHandler handles <ul> and <ol> elements
type ListHandler struct{}

func (l *ListHandler) GetPattern() *regexp.Regexp {
	// This pattern matches both ul and ol
	return regexp.MustCompile(`(?s)<(ul|ol)>(.*?)</(?:ul|ol)>`)
}

func (l *ListHandler) ParseMatched(matches []string) HTMLElement {
	if len(matches) >= 3 {
		listType := matches[1] // "ul" or "ol"
		content := matches[2]
		listItems := l.parseListItems(content)
		return HTMLElement{Tag: listType, Children: listItems}
	}
	return HTMLElement{}
}

func (l *ListHandler) parseListItems(content string) []HTMLElement {
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

func (l *ListHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return widget.renderList(element, x, y, width, indent)
}

// ParagraphHandler handles <p> elements
type ParagraphHandler struct{}

func (p *ParagraphHandler) GetPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<p>(.*?)</p>`)
}

func (p *ParagraphHandler) ParseMatched(matches []string) HTMLElement {
	if len(matches) >= 2 {
		return HTMLElement{Tag: "p", Content: matches[1]}
	}
	return HTMLElement{}
}

func (p *ParagraphHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return widget.renderFormattedText(element.Content, x, y, width)
}

// BreakHandler handles <br> elements
type BreakHandler struct{}

func (b *BreakHandler) GetPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<br\s*/?>`)
}

func (b *BreakHandler) ParseMatched(matches []string) HTMLElement {
	return HTMLElement{Tag: "br"}
}

func (b *BreakHandler) Render(widget *HTMLWidget, element HTMLElement, x, y, width float32, indent int) float32 {
	return y + 20
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

// FontSet manages different font sizes for HTML elements - EXTENDED FOR MONOSPACE
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
	// NEW: Monospace fonts for code
	Monospace      rl.Font // For <code> and <pre>
	MonospaceLarge rl.Font // For larger code blocks
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
	// Text measurement cache
	textCache *TextMeasureCache
	// Element Index for modular element handling
	elementIndex *ElementIndex
}

// NewHTMLWidget creates a new HTML widget - API UNCHANGED
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
		// Initialize text measurement cache with reasonable limit
		textCache: NewTextMeasureCache(1000),
		// Initialize element index with default handlers
		elementIndex: NewElementIndex(),
	}

	// Load fonts using global font manager
	widget.loadFonts()

	// Parse HTML content using element index
	widget.Elements = widget.parseHTML(content)

	return widget
}

// Load TTF fonts at various sizes - NOW WITH MONOSPACE SUPPORT
func (w *HTMLWidget) loadFonts() {
	fm := getFontManager()

	// Get shared fonts from global manager
	w.Fonts = FontSet{
		Regular:    fm.GetFont("arial", 16),
		Bold:       fm.GetFont("arial-bold", 16),
		Italic:     fm.GetFont("arial-italic", 16),
		BoldItalic: fm.GetFont("arial-bold", 16), // Use bold for now
		H1:         fm.GetFont("arial", 32),
		H2:         fm.GetFont("arial", 28),
		H3:         fm.GetFont("arial", 24),
		H4:         fm.GetFont("arial", 20),
		H5:         fm.GetFont("arial", 18),
		H6:         fm.GetFont("arial", 16),
		// NEW: Load monospace fonts
		Monospace:      fm.GetMonospaceFont(14), // Slightly smaller for inline code
		MonospaceLarge: fm.GetMonospaceFont(16), // Standard size for code blocks
	}

	fmt.Printf("Widget loaded fonts from global manager (including monospace)\n")
}

// Cached text measurement helper - replaces direct rl.MeasureTextEx calls
func (w *HTMLWidget) measureText(font rl.Font, text string, fontSize float32) rl.Vector2 {
	return w.textCache.GetTextSize(font, text, fontSize)
}

// Cached text width helper - for common width-only measurements
func (w *HTMLWidget) measureTextWidth(font rl.Font, text string, fontSize float32) float32 {
	return w.textCache.GetTextWidth(font, text, fontSize)
}

// ROBUST HTML parser using Element Index - SIMPLE SEQUENTIAL APPROACH
func (w *HTMLWidget) parseHTML(html string) []HTMLElement {
	var elements []HTMLElement
	html = strings.TrimSpace(html)

	remaining := html

	for len(remaining) > 0 && len(elements) < 1000 { // Safety limit to prevent infinite loops
		// Find the earliest HTML element using Element Index
		elementType, element, newRemaining, found := w.elementIndex.FindFirstElement(remaining)

		if found {
			// Calculate text before this element
			// consumedLength := len(remaining) - len(newRemaining)

			// Find where the actual HTML tag starts
			pattern := w.elementIndex.GetHandler(elementType).GetPattern()
			loc := pattern.FindStringIndex(remaining)

			if loc != nil && loc[0] > 0 {
				// Add text before the HTML tag
				beforeText := strings.TrimSpace(remaining[:loc[0]])
				if beforeText != "" {
					elements = append(elements, HTMLElement{Tag: "text", Content: beforeText})
				}
			}

			// Add the parsed element
			if element.Tag != "" {
				elements = append(elements, element)
			}

			remaining = newRemaining
		} else {
			// No more HTML elements found, add remaining content as text
			if strings.TrimSpace(remaining) != "" {
				elements = append(elements, HTMLElement{Tag: "text", Content: strings.TrimSpace(remaining)})
			}
			break
		}
	}

	return elements
}

// Parse text into segments - FIXED SPACING PRESERVATION
func (w *HTMLWidget) parseTextSegments(text string) []HTMLElement {
	var segments []HTMLElement

	// If no HTML tags, return as simple text
	if !strings.Contains(text, "<") {
		return []HTMLElement{{Tag: "text", Content: text}}
	}

	// Use a single regex that captures all inline formatting tags
	// This pattern will match links, bold, italic, and inline code in order of appearance
	pattern := regexp.MustCompile(`(<a\s+href="([^"]*)">(.*?)</a>|<b>(.*?)</b>|<i>(.*?)</i>|<code>(.*?)</code>)`)

	currentPos := 0
	for {
		match := pattern.FindStringSubmatchIndex(text[currentPos:])
		if match == nil {
			// No more matches, add remaining text
			if currentPos < len(text) {
				remaining := text[currentPos:]
				if remaining != "" {
					segments = append(segments, HTMLElement{Tag: "text", Content: remaining})
				}
			}
			break
		}

		// Adjust match indices to absolute positions
		absoluteStart := currentPos + match[0]
		absoluteEnd := currentPos + match[1]

		// Add text before the match
		if absoluteStart > currentPos {
			beforeText := text[currentPos:absoluteStart]
			if beforeText != "" {
				segments = append(segments, HTMLElement{Tag: "text", Content: beforeText})
			}
		}

		// Extract the full matched text to determine tag type
		fullMatch := text[absoluteStart:absoluteEnd]

		if strings.HasPrefix(fullMatch, "<a href=\"") {
			// Link tag - extract href and content
			submatch := pattern.FindStringSubmatch(text[currentPos:])
			href := submatch[2]
			content := submatch[3]
			segments = append(segments, HTMLElement{Tag: "a", Content: content, Href: href})
		} else if strings.HasPrefix(fullMatch, "<b>") {
			// Bold tag
			submatch := pattern.FindStringSubmatch(text[currentPos:])
			content := submatch[4]
			segments = append(segments, HTMLElement{Tag: "b", Content: content, Bold: true})
		} else if strings.HasPrefix(fullMatch, "<i>") {
			// Italic tag
			submatch := pattern.FindStringSubmatch(text[currentPos:])
			content := submatch[5]
			segments = append(segments, HTMLElement{Tag: "i", Content: content, Italic: true})
		} else if strings.HasPrefix(fullMatch, "<code>") {
			// NEW: Inline code tag
			submatch := pattern.FindStringSubmatch(text[currentPos:])
			content := submatch[6]
			segments = append(segments, HTMLElement{Tag: "code", Content: content})
		}

		// Move past this match
		currentPos = absoluteEnd
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

// Render the HTML widget with stable content height calculation - API UNCHANGED
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

// Element rendering using Element Index - FULLY FUNCTIONAL
func (w *HTMLWidget) renderElement(element HTMLElement, x, y, width float32, indent int) float32 {
	// Use Element Index for all registered handlers
	if handler := w.elementIndex.GetHandler(element.Tag); handler != nil {
		return handler.Render(w, element, x, y, width, indent)
	}

	// Fallback for text elements and any unregistered elements
	switch element.Tag {
	case "text":
		return w.renderText(element.Content, x, y, width, w.Fonts.Regular, rl.Black)
	default:
		// Unknown element, render as text with warning color
		return w.renderText(element.Content, x, y, width, w.Fonts.Regular, rl.Gray)
	}
}

// NEW: Render preformatted code blocks with background and monospace font
func (w *HTMLWidget) renderPreBlock(element HTMLElement, x, y, width float32) float32 {
	if element.Content == "" {
		return y
	}

	// Add spacing before code block
	y += 10

	// Calculate content dimensions
	lines := strings.Split(element.Content, "\n")
	lineHeight := float32(18)
	padding := float32(12)
	blockHeight := float32(len(lines))*lineHeight + 2*padding

	// Draw background rectangle
	backgroundRect := rl.NewRectangle(x, y, width-40, blockHeight)
	rl.DrawRectangleRec(backgroundRect, rl.Color{R: 248, G: 248, B: 248, A: 255})        // Light gray background
	rl.DrawRectangleLinesEx(backgroundRect, 1, rl.Color{R: 220, G: 220, B: 220, A: 255}) // Border

	// Render each line with monospace font
	currentY := y + padding
	for _, line := range lines {
		// Preserve whitespace exactly as it appears
		w.renderTextWithUnicode(line, x+padding, currentY, w.Fonts.MonospaceLarge, rl.Color{R: 40, G: 40, B: 40, A: 255})
		currentY += lineHeight
	}

	return y + blockHeight + 10
}

// NEW: Render inline code with monospace font and subtle background
func (w *HTMLWidget) renderInlineCode(element HTMLElement, x, y, width float32) float32 {
	if element.Content == "" {
		return y
	}

	font := w.Fonts.Monospace
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 14
	}

	// Measure text for background sizing
	textSize := w.measureText(font, element.Content, fontSize)
	padding := float32(4)

	// Draw subtle background
	backgroundRect := rl.NewRectangle(x-padding, y-2, textSize.X+2*padding, textSize.Y+4)
	rl.DrawRectangleRec(backgroundRect, rl.Color{R: 240, G: 240, B: 240, A: 255})
	rl.DrawRectangleLinesEx(backgroundRect, 1, rl.Color{R: 220, G: 220, B: 220, A: 255})

	// Render code text with monospace font
	w.renderTextWithUnicode(element.Content, x, y, font, rl.Color{R: 40, G: 40, B: 40, A: 255})

	return y + textSize.Y + 5
}

// Legacy rendering methods - KEPT FOR BACKWARD COMPATIBILITY

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

// Render regular text with word wrapping and Unicode support - NOW WITH CACHING
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

		// Use cached text measurement instead of direct call
		textWidth := w.measureTextWidth(font, testLine, float32(font.BaseSize))
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
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16 // fallback
	}

	// Check if text contains any Unicode characters
	hasUnicode := false
	for _, r := range text {
		if r >= 128 {
			hasUnicode = true
			break
		}
	}

	// If no Unicode, use the fast path
	if !hasUnicode {
		rl.DrawTextEx(font, text, rl.NewVector2(x, y), fontSize, 1, color)
		return
	}

	// Unicode present - render character by character
	currentX := x
	runes := []rune(text)

	for _, r := range runes {
		if r < 128 {
			// ASCII character - use DrawTextEx
			charStr := string(r)
			// Use cached measurement for character width
			charWidth := w.measureTextWidth(font, charStr, fontSize)
			rl.DrawTextEx(font, charStr, rl.NewVector2(currentX, y), fontSize, 1, color)
			currentX += charWidth
		} else {
			// Unicode character - use DrawTextCodepoint
			rl.DrawTextCodepoint(font, r, rl.NewVector2(currentX, y), fontSize, color)
			// Conservative width estimate for Unicode chars
			charWidth := fontSize * 0.6
			currentX += charWidth
		}
	}
}

// ENHANCED: Render formatted text with inline code support - NOW WITH CACHING
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

	// Render segments preserving exact spacing
	currentX := x
	currentY := y
	lineHeight := float32(20)

	for _, segment := range segments {
		// Skip empty segments
		if segment.Content == "" {
			continue
		}

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
		case "code":
			// NEW: Inline code formatting
			font = w.Fonts.Monospace
			color = rl.Color{R: 40, G: 40, B: 40, A: 255}
		default:
			font = w.Fonts.Regular
			color = rl.Black
		}

		fontSize := float32(font.BaseSize)
		if fontSize == 0 {
			fontSize = 16
		}

		if segment.Tag == "a" {
			// Handle links specially for clickable areas
			// Use cached text measurement
			textWidth := w.measureTextWidth(font, segment.Content, fontSize)

			// Check if link fits on current line
			if currentX+textWidth > x+width-40 && currentX > x {
				currentY += lineHeight
				currentX = x
			}

			// Store link area in document coordinates
			documentY := currentY + w.ScrollY
			bounds := rl.NewRectangle(currentX, documentY, textWidth, fontSize)
			linkArea := LinkArea{Bounds: bounds, URL: segment.Href}
			w.LinkAreas = append(w.LinkAreas, linkArea)

			// Render link
			w.renderTextWithUnicode(segment.Content, currentX, currentY, font, color)

			// Draw underline
			rl.DrawLineEx(
				rl.NewVector2(currentX, currentY+fontSize),
				rl.NewVector2(currentX+textWidth, currentY+fontSize),
				1, color)

			currentX += textWidth
		} else if segment.Tag == "code" {
			// NEW: Handle inline code with background
			textWidth := w.measureTextWidth(font, segment.Content, fontSize)
			padding := float32(2)

			// Check if code fits on current line
			if currentX+textWidth+2*padding > x+width-40 && currentX > x {
				currentY += lineHeight
				currentX = x
			}

			// Draw subtle background for inline code
			backgroundRect := rl.NewRectangle(currentX-padding, currentY-2, textWidth+2*padding, fontSize+4)
			rl.DrawRectangleRec(backgroundRect, rl.Color{R: 240, G: 240, B: 240, A: 255})

			// Render code text
			w.renderTextWithUnicode(segment.Content, currentX, currentY, font, color)
			currentX += textWidth + 2*padding
		} else {
			// For text, bold, and italic - render EXACTLY as parsed preserving all spaces
			// Use cached text measurement
			textWidth := w.measureTextWidth(font, segment.Content, fontSize)

			// Simple line wrapping check
			if currentX+textWidth > x+width-40 && currentX > x {
				currentY += lineHeight
				currentX = x
			}

			// Render the segment content exactly as it exists
			w.renderTextWithUnicode(segment.Content, currentX, currentY, font, color)
			currentX += textWidth
		}
	}

	return currentY + lineHeight + 5
}

// Render clickable hyperlinks with Unicode support - NOW WITH CACHING
func (w *HTMLWidget) renderLink(element HTMLElement, x, y, width float32) float32 {
	font := w.Fonts.Regular
	fontSize := float32(font.BaseSize)

	// Use cached text measurement
	textSize := w.measureText(font, element.Content, fontSize)

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

// Updated font cleanup with monospace support - API UNCHANGED
func (w *HTMLWidget) Unload() {
	fm := getFontManager()

	// Release shared font references instead of unloading directly
	fm.ReleaseFont("arial", 16)        // Regular
	fm.ReleaseFont("arial-bold", 16)   // Bold
	fm.ReleaseFont("arial-italic", 16) // Italic
	fm.ReleaseFont("arial-bold", 16)   // BoldItalic (same as bold)
	fm.ReleaseFont("arial", 32)        // H1
	fm.ReleaseFont("arial", 28)        // H2
	fm.ReleaseFont("arial", 24)        // H3
	fm.ReleaseFont("arial", 20)        // H4
	fm.ReleaseFont("arial", 18)        // H5
	fm.ReleaseFont("arial", 16)        // H6 (same as regular)

	// NEW: Release monospace fonts
	fm.ReleaseMonospaceFont(14) // Monospace
	fm.ReleaseMonospaceFont(16) // MonospaceLarge

	// Clear text cache to free memory
	if w.textCache != nil {
		w.textCache.Clear()
	}
}

// EXTENSION API - New functionality for adding custom element handlers
// These methods extend the widget without breaking existing API

// RegisterElementHandler allows adding custom element handlers - EXTENSION API
func (w *HTMLWidget) RegisterElementHandler(elementType string, handler ElementHandler) {
	w.elementIndex.RegisterHandler(elementType, handler)
}

// GetElementIndex returns the widget's element index for advanced usage - EXTENSION API
func (w *HTMLWidget) GetElementIndex() *ElementIndex {
	return w.elementIndex
}
