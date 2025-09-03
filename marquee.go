package marquee

import (
	"fmt"
	"strconv"
	"strings"

	//	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type HTMLWidget struct {
	Content        string
	Elements       []HTMLElement
	ScrollY        float32
	TargetScrollY  float32
	TotalHeight    float32
	WidgetHeight   float32
	Fonts          FontSet
	LinkAreas      []LinkArea
	ScrollbarAlpha float32
	LastScrollTime float32
	BodyMargin     float32
	BodyBorder     float32
	BodyPadding    float32
	OnLinkClick    func(string)

	document  HTMLDocument
	parser    *StateMachineParser
	renderer  *HTMLRenderer
	textCache *TextMeasureCache

	linkAreaPool []LinkArea
	poolCapacity int
}

func NewHTMLWidget(content string) *HTMLWidget {
	widget := &HTMLWidget{
		Content:        content,
		LinkAreas:      make([]LinkArea, 0),
		ScrollbarAlpha: 1.0,
		LastScrollTime: 0.0,
		BodyMargin:     10.0,
		BodyBorder:     1.0,
		BodyPadding:    15.0,
		textCache:      NewTextMeasureCache(1000),
		parser:         NewStateMachineParser(),
		renderer:       NewHTMLRenderer(),

		linkAreaPool: make([]LinkArea, 100),
		poolCapacity: 100,
	}

	widget.loadFonts()
	widget.parseHTML(content)

	return widget
}

func (w *HTMLWidget) loadFonts() {
	fm := getFontManager()

	w.Fonts = FontSet{
		Regular:        fm.GetFont("arial", 16),
		Bold:           fm.GetFont("arial-bold", 16),
		Italic:         fm.GetFont("arial-italic", 16),
		BoldItalic:     fm.GetFont("arial-bold", 16),
		H1:             fm.GetFont("arial", 32),
		H2:             fm.GetFont("arial", 28),
		H3:             fm.GetFont("arial", 24),
		H4:             fm.GetFont("arial", 20),
		H5:             fm.GetFont("arial", 18),
		H6:             fm.GetFont("arial", 16),
		Monospace:      fm.GetMonospaceFont(14),
		MonospaceLarge: fm.GetMonospaceFont(16),
	}

	if !fm.GetFontStatus("arial", 16) {
		fmt.Printf("Warning: Regular font failed to load, using system default\n")
	}
	if !fm.GetFontStatus("arial-bold", 16) {
		fmt.Printf("Warning: Bold font failed to load, formatting may be limited\n")
	}
	if !fm.GetFontStatus("arial-italic", 16) {
		fmt.Printf("Warning: Italic font failed to load, formatting may be limited\n")
	}
}

func (w *HTMLWidget) measureText(font rl.Font, text string, fontSize float32) rl.Vector2 {
	return w.textCache.GetTextSize(font, text, fontSize)
}

func (w *HTMLWidget) measureTextWidth(font rl.Font, text string, fontSize float32) float32 {
	return w.textCache.GetTextWidth(font, text, fontSize)
}

func (w *HTMLWidget) parseHTML(html string) {
	w.document = w.parser.Parse(html)

	w.Elements = w.createLegacyElementsForAPI()
}

func (w *HTMLWidget) createLegacyElementsForAPI() []HTMLElement {
	var elements []HTMLElement
	for _, node := range w.document.Root.Children {
		elements = append(elements, w.nodeToLegacyElement(node))
	}
	return elements
}

func (w *HTMLWidget) nodeToLegacyElement(node HTMLNode) HTMLElement {
	element := HTMLElement{Tag: node.Tag, Content: node.Content}

	if href, exists := node.Attributes["href"]; exists {
		element.Href = href
	}

	if strings.HasPrefix(node.Tag, "h") && len(node.Tag) == 2 {
		if level, err := strconv.Atoi(node.Tag[1:]); err == nil {
			element.Level = level
		}
	}

	if node.Tag == "span" {
		if style, exists := node.Attributes["style"]; exists {
			element.Bold = strings.Contains(style, "font-weight: bold")
			element.Italic = strings.Contains(style, "font-style: italic")
		}
	}

	for _, child := range node.Children {
		element.Children = append(element.Children, w.nodeToLegacyElement(child))
	}

	return element
}

func (w *HTMLWidget) renderTextWithUnicode(text string, x, y float32, font rl.Font, color rl.Color) {
	renderTextWithUnicode(text, x, y, font, color)
}

func (w *HTMLWidget) renderText(text string, x, y, width float32, font rl.Font, color rl.Color) float32 {
	if text == "" {
		return y
	}

	ph := &ParagraphRenderHandler{}
	words := ph.intelligentWordSplit(text)
	currentLine := ""
	lineHeight := float32(20)
	currentY := y

	rightMargin := w.BodyMargin + w.BodyPadding

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		textWidth := w.measureTextWidth(font, testLine, float32(font.BaseSize))

		if textWidth > width-rightMargin && currentLine != "" {
			w.renderTextWithUnicode(currentLine, x, currentY, font, color)
			currentY += lineHeight
			currentLine = word
		} else {
			currentLine = testLine
		}
	}

	if currentLine != "" {
		w.renderTextWithUnicode(currentLine, x, currentY, font, color)
		currentY += lineHeight
	}

	return currentY + 5
}

func (w *HTMLWidget) Update() {
	rl.SetMouseCursor(rl.MouseCursorDefault)

	wheel := rl.GetMouseWheelMove()
	w.ScrollY -= wheel * 20

	if w.ScrollY < 0 {
		w.ScrollY = 0
	}

	widgetHeight := float32(650)
	maxScroll := w.TotalHeight - widgetHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	if w.ScrollY > maxScroll {
		w.ScrollY = maxScroll
	}

	mousePos := rl.GetMousePosition()
	for i := range w.LinkAreas {
		area := &w.LinkAreas[i]

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

func (w *HTMLWidget) Render(x, y, width, height float32) {

	if cap(w.LinkAreas) < w.poolCapacity {
		w.LinkAreas = make([]LinkArea, 0, w.poolCapacity)
	} else {
		w.LinkAreas = w.LinkAreas[:0]
	}
	w.WidgetHeight = height

	rl.DrawRectangle(int32(x), int32(y), int32(width), int32(height), rl.White)

	if w.BodyBorder > 0 {
		borderColor := rl.Color{R: 200, G: 200, B: 200, A: 255}
		rl.DrawRectangleLinesEx(
			rl.NewRectangle(x, y, width, height),
			w.BodyBorder,
			borderColor)
	}

	contentX := x + w.BodyMargin + w.BodyPadding
	contentY := y + w.BodyMargin + w.BodyPadding - w.ScrollY
	contentWidth := width - 2*(w.BodyMargin+w.BodyPadding)

	rl.BeginScissorMode(int32(x), int32(y), int32(width), int32(height))

	ctx := RenderContext{
		X:           contentX,
		Y:           contentY,
		Width:       contentWidth,
		ParentFont:  w.Fonts.Regular,
		ParentColor: rl.Black,
		Widget:      w,
		CurrentX:    contentX,
	}

	result := w.renderer.RenderDocument(w.document, ctx)

	w.TotalHeight = result.NextY + w.ScrollY - contentY + 2*(w.BodyMargin+w.BodyPadding)

	for _, linkArea := range result.LinkAreas {

		screenArea := linkArea
		screenArea.Bounds.Y += w.ScrollY
		w.LinkAreas = append(w.LinkAreas, screenArea)
	}

	rl.EndScissorMode()

	if w.TotalHeight > height {
		w.drawScrollbar(x, y, width, height)
	}
}

func (w *HTMLWidget) drawScrollbar(x, y, width, height float32) {
	if w.TotalHeight <= height || w.ScrollbarAlpha <= 0.01 {
		return
	}

	scrollbarWidth := float32(10)

	contentMargin := w.BodyMargin + w.BodyPadding
	scrollbarX := x + width - scrollbarWidth - contentMargin

	contentArea := height - 2*contentMargin
	trackY := y + contentMargin
	thumbHeight := contentArea * 0.2

	if thumbHeight < 40 {
		thumbHeight = 40
	}
	if thumbHeight > contentArea*0.8 {
		thumbHeight = contentArea * 0.8
	}

	maxScroll := w.TotalHeight - height
	if maxScroll <= 0 {
		return
	}

	scrollProgress := w.ScrollY / maxScroll
	if scrollProgress < 0 {
		scrollProgress = 0
	}
	if scrollProgress > 1 {
		scrollProgress = 1
	}

	trackHeight := contentArea - thumbHeight
	thumbY := trackY + scrollProgress*trackHeight

	alpha := uint8(w.ScrollbarAlpha * 120)
	thumbColor := rl.Color{R: 60, G: 60, B: 60, A: alpha}

	rl.DrawRectangle(
		int32(scrollbarX),
		int32(thumbY),
		int32(scrollbarWidth),
		int32(thumbHeight),
		thumbColor)
}

func (w *HTMLWidget) Unload() {
	fm := getFontManager()

	fm.ReleaseFont("arial", 16)
	fm.ReleaseFont("arial-bold", 16)
	fm.ReleaseFont("arial-italic", 16)
	fm.ReleaseFont("arial-bold", 16)
	fm.ReleaseFont("arial", 32)
	fm.ReleaseFont("arial", 28)
	fm.ReleaseFont("arial", 24)
	fm.ReleaseFont("arial", 20)
	fm.ReleaseFont("arial", 18)
	fm.ReleaseFont("arial", 16)

	fm.ReleaseMonospaceFont(14)
	fm.ReleaseMonospaceFont(16)

	if w.textCache != nil {
		w.textCache.Clear()
	}
}

func (w *HTMLWidget) RegisterRenderHandler(elementType string, handler RenderHandler) {
	w.renderer.RegisterHandler(elementType, handler)
}

func (w *HTMLWidget) GetRenderer() *HTMLRenderer {
	return w.renderer
}

func (w *HTMLWidget) GetDocument() HTMLDocument {
	return w.document
}

func (w *HTMLWidget) DebugDocument() {
	fmt.Println("=== MARQUEE DEBUG: Document Structure ===")
	w.debugNode(w.document.Root, 0)
	fmt.Println("=== End Document Structure ===")
}

func (w *HTMLWidget) debugNode(node HTMLNode, depth int) {
	indent := strings.Repeat("  ", depth)

	if node.Type == NodeTypeText {
		content := strings.TrimSpace(node.Content)
		if content != "" {
			fmt.Printf("%sTEXT: '%s'\n", indent, content)
		}
	} else {
		fmt.Printf("%s<%s", indent, node.Tag)
		for k, v := range node.Attributes {
			fmt.Printf(" %s=\"%s\"", k, v)
		}
		fmt.Printf("> [context=%v]\n", node.Context)

		for _, child := range node.Children {
			w.debugNode(child, depth+1)
		}
	}
}

func (w *HTMLWidget) DebugFonts() {
	fmt.Println("=== MARQUEE DEBUG: Font Status ===")
	fm := getFontManager()

	fonts := []struct {
		name string
		size int32
	}{
		{"arial", 16}, {"arial-bold", 16}, {"arial-italic", 16},
		{"arial", 32}, {"arial", 28}, {"arial", 24}, {"arial", 20}, {"arial", 18},
	}

	for _, f := range fonts {
		status := "✅ Loaded"
		if !fm.GetFontStatus(f.name, f.size) {
			status = "⚠ Fallback"
		}
		fmt.Printf("  %s %dpx: %s\n", f.name, f.size, status)
	}

	monoStatus := "✅ Loaded"
	if !fm.GetFontStatus("monospace", 14) {
		monoStatus = "⚠ Fallback"
	}
	fmt.Printf("  monospace 14px: %s\n", monoStatus)
	fmt.Println("=== End Font Status ===")
}
