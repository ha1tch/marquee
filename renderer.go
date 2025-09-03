package marquee

import (
	"fmt"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type RenderContext struct {
	X, Y, Width float32
	ParentFont  rl.Font
	ParentColor rl.Color
	Indent      int
	LineHeight  float32
	Widget      *HTMLWidget

	CurrentX      float32
	MaxLineHeight float32
}

type RenderResult struct {
	NextY     float32
	LinkAreas []LinkArea
	Height    float32

	NextX      float32
	LineHeight float32
}

type RenderHandler interface {
	CanRender(node HTMLNode) bool
	Render(node HTMLNode, ctx RenderContext) RenderResult
}

type HTMLRenderer struct {
	handlers map[string]RenderHandler
}

func NewHTMLRenderer() *HTMLRenderer {
	r := &HTMLRenderer{
		handlers: make(map[string]RenderHandler),
	}

	r.RegisterHandler("text", &TextRenderHandler{})
	r.RegisterHandler("span", &SpanRenderHandler{})
	r.RegisterHandler("a", &LinkRenderHandler{})
	r.RegisterHandler("h1", &HeadingRenderHandler{})
	r.RegisterHandler("h2", &HeadingRenderHandler{})
	r.RegisterHandler("h3", &HeadingRenderHandler{})
	r.RegisterHandler("h4", &HeadingRenderHandler{})
	r.RegisterHandler("h5", &HeadingRenderHandler{})
	r.RegisterHandler("h6", &HeadingRenderHandler{})
	r.RegisterHandler("p", &ParagraphRenderHandler{})
	r.RegisterHandler("ul", &ListRenderHandler{})
	r.RegisterHandler("ol", &ListRenderHandler{})

	r.RegisterHandler("hr", &HRRenderHandler{})
	r.RegisterHandler("br", &BreakRenderHandler{})
	r.RegisterHandler("pre", &PreRenderHandler{})
	r.RegisterHandler("code", &CodeRenderHandler{})

	return r
}

func (r *HTMLRenderer) RegisterHandler(tag string, handler RenderHandler) {
	r.handlers[tag] = handler
}

func (r *HTMLRenderer) RenderNode(node HTMLNode, ctx RenderContext) RenderResult {
	if handler, exists := r.handlers[node.Tag]; exists && handler.CanRender(node) {
		return handler.Render(node, ctx)
	}

	return r.handlers["text"].Render(node, ctx)
}

func (r *HTMLRenderer) RenderDocument(document HTMLDocument, ctx RenderContext) RenderResult {
	result := RenderResult{NextY: ctx.Y}

	for _, child := range document.Root.Children {
		childResult := r.RenderNode(child, ctx)
		ctx.Y = childResult.NextY
		result.NextY = childResult.NextY
		result.LinkAreas = append(result.LinkAreas, childResult.LinkAreas...)
	}

	result.Height = result.NextY - ctx.Y
	return result
}

type TextRenderHandler struct{}

func (h *TextRenderHandler) CanRender(node HTMLNode) bool {
	return node.Type == NodeTypeText || node.Tag == "text"
}

func (h *TextRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	content := node.Content
	if content == "" {
		return RenderResult{NextY: ctx.Y}
	}

	nextY := ctx.Widget.renderText(content, ctx.X, ctx.Y, ctx.Width, ctx.ParentFont, ctx.ParentColor)
	return RenderResult{
		NextY:  nextY,
		Height: nextY - ctx.Y,
	}
}

type SpanRenderHandler struct{}

func (h *SpanRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "span"
}

func (h *SpanRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {

	font := ctx.ParentFont
	color := ctx.ParentColor

	if style, exists := node.Attributes["style"]; exists {
		if strings.Contains(style, "font-weight: bold") {
			font = ctx.Widget.Fonts.Bold
			color = rl.DarkBlue
		}
		if strings.Contains(style, "font-style: italic") {
			font = ctx.Widget.Fonts.Italic
			color = rl.DarkGreen
		}
	}

	if node.Context == ContextInline {

		var content strings.Builder
		for _, child := range node.Children {
			if child.Type == NodeTypeText {
				content.WriteString(child.Content)
			}
		}

		if content.Len() == 0 {
			return RenderResult{NextY: ctx.Y, NextX: ctx.CurrentX}
		}

		text := content.String()
		fontSize := float32(font.BaseSize)
		if fontSize == 0 {
			fontSize = 16
		}

		ctx.Widget.renderTextWithUnicode(text, ctx.CurrentX, ctx.Y, font, color)

		textWidth := ctx.Widget.measureTextWidth(font, text, fontSize)

		return RenderResult{
			NextY:      ctx.Y,
			NextX:      ctx.CurrentX + textWidth,
			Height:     fontSize,
			LineHeight: fontSize,
		}
	}

	return h.renderBlockChildren(node, ctx, font, color)
}

func (h *SpanRenderHandler) renderBlockChildren(node HTMLNode, ctx RenderContext, font rl.Font, color rl.Color) RenderResult {
	childCtx := ctx
	childCtx.ParentFont = font
	childCtx.ParentColor = color

	result := RenderResult{NextY: ctx.Y}

	for _, child := range node.Children {
		childResult := ctx.Widget.renderer.RenderNode(child, childCtx)
		childCtx.Y = childResult.NextY
		result.NextY = childResult.NextY
		result.LinkAreas = append(result.LinkAreas, childResult.LinkAreas...)
	}

	result.Height = result.NextY - ctx.Y
	return result
}

type LinkRenderHandler struct{}

func (h *LinkRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "a"
}

func (h *LinkRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	href, _ := node.Attributes["href"]

	var content strings.Builder
	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			content.WriteString(child.Content)
		}
	}

	if content.Len() == 0 {
		return RenderResult{NextY: ctx.Y, NextX: ctx.CurrentX}
	}

	font := ctx.ParentFont
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16
	}

	text := content.String()
	textSize := ctx.Widget.measureText(font, text, fontSize)

	color := rl.Blue

	bounds := rl.NewRectangle(ctx.CurrentX, ctx.Y, textSize.X, textSize.Y)
	linkArea := LinkArea{Bounds: bounds, URL: href}

	if node.Context == ContextInline {

		ctx.Widget.renderTextWithUnicode(text, ctx.CurrentX, ctx.Y, font, color)

		rl.DrawLineEx(
			rl.NewVector2(ctx.CurrentX, ctx.Y+textSize.Y),
			rl.NewVector2(ctx.CurrentX+textSize.X, ctx.Y+textSize.Y),
			1, color)

		return RenderResult{
			NextY:      ctx.Y,
			NextX:      ctx.CurrentX + textSize.X,
			LinkAreas:  []LinkArea{linkArea},
			Height:     textSize.Y,
			LineHeight: textSize.Y,
		}
	} else {

		ctx.Widget.renderTextWithUnicode(text, ctx.X, ctx.Y, font, color)

		rl.DrawLineEx(
			rl.NewVector2(ctx.X, ctx.Y+textSize.Y),
			rl.NewVector2(ctx.X+textSize.X, ctx.Y+textSize.Y),
			1, color)

		return RenderResult{
			NextY:     ctx.Y + textSize.Y + 5,
			LinkAreas: []LinkArea{linkArea},
			Height:    textSize.Y + 5,
		}
	}
}

type HeadingRenderHandler struct{}

func (h *HeadingRenderHandler) CanRender(node HTMLNode) bool {
	return strings.HasPrefix(node.Tag, "h") && len(node.Tag) == 2
}

func (h *HeadingRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	level, _ := strconv.Atoi(node.Tag[1:])

	var font rl.Font
	switch level {
	case 1:
		font = ctx.Widget.Fonts.H1
	case 2:
		font = ctx.Widget.Fonts.H2
	case 3:
		font = ctx.Widget.Fonts.H3
	case 4:
		font = ctx.Widget.Fonts.H4
	case 5:
		font = ctx.Widget.Fonts.H5
	case 6:
		font = ctx.Widget.Fonts.H6
	default:
		font = ctx.Widget.Fonts.Regular
	}

	spacingBefore := float32([]int{25, 20, 18, 15, 12, 10}[level-1])
	spacingAfter := float32([]int{15, 12, 10, 8, 6, 5}[level-1])

	y := ctx.Y + spacingBefore

	var content strings.Builder
	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			content.WriteString(child.Content)
		}
	}

	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = float32([]int{32, 28, 24, 20, 18, 16}[level-1])
	}

	ctx.Widget.renderTextWithUnicode(content.String(), ctx.X, y, font, rl.DarkBlue)

	return RenderResult{
		NextY:  y + fontSize + spacingAfter,
		Height: fontSize + spacingBefore + spacingAfter,
	}
}

type ParagraphRenderHandler struct{}

func (h *ParagraphRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "p"
}

func (h *ParagraphRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {

	if ctx.ParentFont.BaseSize == 0 {
		ctx.ParentFont = ctx.Widget.Fonts.Regular
	}
	if ctx.ParentColor.R == 0 && ctx.ParentColor.G == 0 && ctx.ParentColor.B == 0 && ctx.ParentColor.A == 0 {
		ctx.ParentColor = rl.Black
	}

	segments := h.buildInlineSegments(node, ctx)

	return h.renderSegmentsWithWrapping(segments, ctx)
}

func (h *ParagraphRenderHandler) buildInlineSegments(node HTMLNode, ctx RenderContext) []inlineSegment {
	var segments []inlineSegment

	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			segments = append(segments, inlineSegment{
				text:  child.Content,
				font:  ctx.ParentFont,
				color: ctx.ParentColor,
			})
		} else if child.Type == NodeTypeElement && child.Context == ContextInline {
			childSegments := h.getSegmentsFromElement(child, ctx)
			segments = append(segments, childSegments...)
		}
	}

	return segments
}

func (h *ParagraphRenderHandler) getSegmentsFromElement(node HTMLNode, ctx RenderContext) []inlineSegment {
	font := ctx.ParentFont
	color := ctx.ParentColor

	if node.Tag == "span" {
		if style, exists := node.Attributes["style"]; exists {
			if strings.Contains(style, "font-weight: bold") {
				font = ctx.Widget.Fonts.Bold
				color = rl.DarkBlue
			}
			if strings.Contains(style, "font-style: italic") {
				font = ctx.Widget.Fonts.Italic
				color = rl.DarkGreen
			}
		}
	} else if node.Tag == "a" {

		color = rl.Blue
	}

	var segments []inlineSegment
	href, _ := node.Attributes["href"]

	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			segments = append(segments, inlineSegment{
				text:  child.Content,
				font:  font,
				color: color,
				href:  href,
			})
		} else if child.Type == NodeTypeElement && child.Context == ContextInline {

			nestedCtx := ctx
			nestedCtx.ParentFont = font
			nestedCtx.ParentColor = color
			nestedSegments := h.getSegmentsFromElement(child, nestedCtx)
			segments = append(segments, nestedSegments...)
		}
	}

	return segments
}

func (h *ParagraphRenderHandler) renderSegmentsWithWrapping(segments []inlineSegment, ctx RenderContext) RenderResult {
	result := RenderResult{NextY: ctx.Y}

	currentY := ctx.Y
	lineHeight := float32(20)
	currentLineSegments := []inlineSegment{}
	currentLineWidth := float32(0)

	rightMargin := ctx.Widget.BodyMargin + ctx.Widget.BodyPadding

	for _, segment := range segments {

		words := h.intelligentWordSplit(segment.text)

		for _, word := range words {
			wordSegment := inlineSegment{
				text:  word,
				font:  segment.font,
				color: segment.color,
				href:  segment.href,
			}

			wordWidth := ctx.Widget.measureTextWidth(segment.font, word, float32(segment.font.BaseSize))

			availableWidth := ctx.Width - rightMargin
			if currentLineWidth+wordWidth > availableWidth && len(currentLineSegments) > 0 {

				h.renderInlineSegments(currentLineSegments, ctx.X, currentY, ctx.Widget, &result)

				currentY += lineHeight
				currentLineSegments = []inlineSegment{wordSegment}
				currentLineWidth = wordWidth
			} else {

				if len(currentLineSegments) > 0 {
					spaceSegment := inlineSegment{
						text:  " ",
						font:  segment.font,
						color: segment.color,
					}
					currentLineSegments = append(currentLineSegments, spaceSegment)
					currentLineWidth += ctx.Widget.measureTextWidth(segment.font, " ", float32(segment.font.BaseSize))
				}

				currentLineSegments = append(currentLineSegments, wordSegment)
				currentLineWidth += wordWidth
			}
		}
	}

	if len(currentLineSegments) > 0 {
		h.renderInlineSegments(currentLineSegments, ctx.X, currentY, ctx.Widget, &result)
		currentY += lineHeight
	}

	result.NextY = currentY + 5
	result.Height = result.NextY - ctx.Y
	return result
}

func (h *ParagraphRenderHandler) intelligentWordSplit(text string) []string {
	words := strings.Fields(text)
	var result []string

	for _, word := range words {

		if len(word) > 40 && (strings.Contains(word, "://") || strings.Contains(word, ".com") || strings.Contains(word, ".org") || strings.Contains(word, "/")) {

			breakPoints := []string{"/", "?", "&", "=", ".", "-"}
			segments := h.splitAtBreakPoints(word, breakPoints)
			result = append(result, segments...)
		} else if len(word) > 30 {

			for i := 0; i < len(word); i += 25 {
				end := i + 25
				if end > len(word) {
					end = len(word)
				}
				result = append(result, word[i:end])
			}
		} else {
			result = append(result, word)
		}
	}

	return result
}

func (h *ParagraphRenderHandler) splitAtBreakPoints(text string, breakPoints []string) []string {
	segments := []string{text}

	for _, breakPoint := range breakPoints {
		var newSegments []string
		for _, segment := range segments {
			if len(segment) > 30 && strings.Contains(segment, breakPoint) {
				parts := strings.Split(segment, breakPoint)
				for i, part := range parts {
					if i > 0 {
						newSegments = append(newSegments, breakPoint+part)
					} else {
						newSegments = append(newSegments, part)
					}
				}
			} else {
				newSegments = append(newSegments, segment)
			}
		}
		segments = newSegments
	}

	return segments
}

func (h *ParagraphRenderHandler) renderInlineSegments(segments []inlineSegment, x, y float32, widget *HTMLWidget, result *RenderResult) {
	currentX := x

	for _, segment := range segments {
		widget.renderTextWithUnicode(segment.text, currentX, y, segment.font, segment.color)

		segmentWidth := widget.measureTextWidth(segment.font, segment.text, float32(segment.font.BaseSize))

		if segment.href != "" {
			fontSize := float32(segment.font.BaseSize)
			if fontSize == 0 {
				fontSize = 16
			}

			bounds := rl.NewRectangle(currentX, y, segmentWidth, fontSize)
			linkArea := LinkArea{Bounds: bounds, URL: segment.href}
			result.LinkAreas = append(result.LinkAreas, linkArea)

			rl.DrawLineEx(
				rl.NewVector2(currentX, y+fontSize),
				rl.NewVector2(currentX+segmentWidth, y+fontSize),
				1, segment.color)
		}

		currentX += segmentWidth
	}
}

type ListRenderHandler struct{}

func (h *ListRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "ul" || node.Tag == "ol" || node.Tag == "li"
}

func (h *ListRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	if node.Tag == "li" {

		return h.renderListItem(node, ctx, "ul", 0)
	}

	result := RenderResult{NextY: ctx.Y + 10}

	currentY := result.NextY
	for i, child := range node.Children {
		if child.Tag == "li" {

			childCtx := ctx
			baseIndent := float32(25)
			nestedIndent := float32(ctx.Indent * 20)
			childCtx.X = ctx.X + baseIndent + nestedIndent
			childCtx.Y = currentY
			childCtx.Width = ctx.Width - baseIndent - nestedIndent - ctx.Widget.BodyMargin
			childCtx.Indent = ctx.Indent + 1
			childCtx.ParentFont = ctx.ParentFont
			childCtx.ParentColor = ctx.ParentColor

			listItemResult := h.renderListItem(child, childCtx, node.Tag, i)
			currentY = listItemResult.NextY
			result.NextY = listItemResult.NextY
			result.LinkAreas = append(result.LinkAreas, listItemResult.LinkAreas...)
		}
	}

	result.Height = result.NextY - ctx.Y
	return result
}

func (h *ListRenderHandler) renderListItem(node HTMLNode, ctx RenderContext, listType string, index int) RenderResult {

	bulletFont := ctx.Widget.Fonts.Regular
	if listType == "ol" {
		marker := fmt.Sprintf("%d.", index+1)
		rl.DrawTextEx(bulletFont, marker, rl.NewVector2(ctx.X-20, ctx.Y), 16, 1, rl.Black)
	} else {

		bulletRune := rune(0x2022)
		bulletStr := string(bulletRune)
		rl.DrawTextEx(bulletFont, bulletStr, rl.NewVector2(ctx.X-15, ctx.Y), 18, 1, rl.Black)
	}

	contentCtx := ctx
	contentCtx.CurrentX = ctx.X

	if contentCtx.ParentFont.BaseSize == 0 {
		contentCtx.ParentFont = ctx.Widget.Fonts.Regular
	}
	if contentCtx.ParentColor.R == 0 && contentCtx.ParentColor.G == 0 && contentCtx.ParentColor.B == 0 && contentCtx.ParentColor.A == 0 {
		contentCtx.ParentColor = rl.Black
	}

	return h.renderListItemContent(node, contentCtx)
}

func (h *ListRenderHandler) renderListItemContent(node HTMLNode, ctx RenderContext) RenderResult {

	segments := h.buildListItemSegments(node, ctx)
	return h.renderListItemSegments(segments, ctx)
}

func (h *ListRenderHandler) buildListItemSegments(node HTMLNode, ctx RenderContext) []inlineSegment {
	var segments []inlineSegment

	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			segments = append(segments, inlineSegment{
				text:  child.Content,
				font:  ctx.ParentFont,
				color: ctx.ParentColor,
			})
		} else if child.Type == NodeTypeElement && child.Context == ContextInline {
			childSegments := h.getListItemSegmentsFromElement(child, ctx)
			segments = append(segments, childSegments...)
		}
	}

	return segments
}

func (h *ListRenderHandler) getListItemSegmentsFromElement(node HTMLNode, ctx RenderContext) []inlineSegment {
	font := ctx.ParentFont
	color := ctx.ParentColor

	if node.Tag == "span" {
		if style, exists := node.Attributes["style"]; exists {
			if strings.Contains(style, "font-weight: bold") {
				font = ctx.Widget.Fonts.Bold
				color = rl.DarkBlue
			}
			if strings.Contains(style, "font-style: italic") {
				font = ctx.Widget.Fonts.Italic
				color = rl.DarkGreen
			}
		}
	} else if node.Tag == "a" {

		color = rl.Blue
	}

	var segments []inlineSegment
	href, _ := node.Attributes["href"]

	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			segments = append(segments, inlineSegment{
				text:  child.Content,
				font:  font,
				color: color,
				href:  href,
			})
		} else if child.Type == NodeTypeElement && child.Context == ContextInline {

			nestedCtx := ctx
			nestedCtx.ParentFont = font
			nestedCtx.ParentColor = color
			nestedSegments := h.getListItemSegmentsFromElement(child, nestedCtx)
			segments = append(segments, nestedSegments...)
		}
	}

	return segments
}

func (h *ListRenderHandler) renderListItemSegments(segments []inlineSegment, ctx RenderContext) RenderResult {
	result := RenderResult{NextY: ctx.Y}

	currentY := ctx.Y
	lineHeight := float32(20)
	currentLineSegments := []inlineSegment{}
	currentLineWidth := float32(0)

	rightMargin := ctx.Widget.BodyMargin + ctx.Widget.BodyPadding

	for _, segment := range segments {

		words := h.intelligentWordSplit(segment.text)

		for _, word := range words {
			wordSegment := inlineSegment{
				text:  word,
				font:  segment.font,
				color: segment.color,
				href:  segment.href,
			}

			wordWidth := ctx.Widget.measureTextWidth(segment.font, word, float32(segment.font.BaseSize))

			availableWidth := ctx.Width - rightMargin
			if currentLineWidth+wordWidth > availableWidth && len(currentLineSegments) > 0 {

				h.renderListItemInlineSegments(currentLineSegments, ctx.X, currentY, ctx.Widget, &result)

				currentY += lineHeight
				currentLineSegments = []inlineSegment{wordSegment}
				currentLineWidth = wordWidth
			} else {

				if len(currentLineSegments) > 0 {
					spaceSegment := inlineSegment{
						text:  " ",
						font:  segment.font,
						color: segment.color,
					}
					currentLineSegments = append(currentLineSegments, spaceSegment)
					currentLineWidth += ctx.Widget.measureTextWidth(segment.font, " ", float32(segment.font.BaseSize))
				}

				currentLineSegments = append(currentLineSegments, wordSegment)
				currentLineWidth += wordWidth
			}
		}
	}

	if len(currentLineSegments) > 0 {
		h.renderListItemInlineSegments(currentLineSegments, ctx.X, currentY, ctx.Widget, &result)
		currentY += lineHeight
	}

	result.NextY = currentY + 5
	result.Height = result.NextY - ctx.Y
	return result
}

func (h *ListRenderHandler) intelligentWordSplit(text string) []string {
	words := strings.Fields(text)
	var result []string

	for _, word := range words {

		if len(word) > 40 && (strings.Contains(word, "://") || strings.Contains(word, ".com") || strings.Contains(word, ".org") || strings.Contains(word, "/")) {

			breakPoints := []string{"/", "?", "&", "=", ".", "-"}
			segments := h.splitAtBreakPoints(word, breakPoints)
			result = append(result, segments...)
		} else if len(word) > 30 {

			for i := 0; i < len(word); i += 25 {
				end := i + 25
				if end > len(word) {
					end = len(word)
				}
				result = append(result, word[i:end])
			}
		} else {
			result = append(result, word)
		}
	}

	return result
}

func (h *ListRenderHandler) splitAtBreakPoints(text string, breakPoints []string) []string {
	segments := []string{text}

	for _, breakPoint := range breakPoints {
		var newSegments []string
		for _, segment := range segments {
			if len(segment) > 30 && strings.Contains(segment, breakPoint) {
				parts := strings.Split(segment, breakPoint)
				for i, part := range parts {
					if i > 0 {
						newSegments = append(newSegments, breakPoint+part)
					} else {
						newSegments = append(newSegments, part)
					}
				}
			} else {
				newSegments = append(newSegments, segment)
			}
		}
		segments = newSegments
	}

	return segments
}

func (h *ListRenderHandler) renderListItemInlineSegments(segments []inlineSegment, x, y float32, widget *HTMLWidget, result *RenderResult) {
	currentX := x

	for _, segment := range segments {
		widget.renderTextWithUnicode(segment.text, currentX, y, segment.font, segment.color)

		segmentWidth := widget.measureTextWidth(segment.font, segment.text, float32(segment.font.BaseSize))

		if segment.href != "" {
			fontSize := float32(segment.font.BaseSize)
			if fontSize == 0 {
				fontSize = 16
			}

			bounds := rl.NewRectangle(currentX, y, segmentWidth, fontSize)
			linkArea := LinkArea{Bounds: bounds, URL: segment.href}
			result.LinkAreas = append(result.LinkAreas, linkArea)

			rl.DrawLineEx(
				rl.NewVector2(currentX, y+fontSize),
				rl.NewVector2(currentX+segmentWidth, y+fontSize),
				1, segment.color)
		}

		currentX += segmentWidth
	}
}

type HRRenderHandler struct{}

func (h *HRRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "hr"
}

func (h *HRRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	y := ctx.Y + 10

	rightMargin := ctx.Widget.BodyMargin + ctx.Widget.BodyPadding
	lineWidth := ctx.Width - rightMargin

	rl.DrawLineEx(
		rl.NewVector2(ctx.X, y),
		rl.NewVector2(ctx.X+lineWidth, y),
		2, rl.Gray)

	return RenderResult{
		NextY:  y + 15,
		Height: 25,
	}
}

type BreakRenderHandler struct{}

func (h *BreakRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "br"
}

func (h *BreakRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	return RenderResult{
		NextY:  ctx.Y + 20,
		Height: 20,
	}
}

type PreRenderHandler struct{}

func (h *PreRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "pre"
}

func (h *PreRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {

	var content strings.Builder
	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			content.WriteString(child.Content)
		}
	}

	if content.Len() == 0 {
		return RenderResult{NextY: ctx.Y}
	}

	y := ctx.Y + 10
	lines := strings.Split(content.String(), "\n")
	lineHeight := float32(18)
	padding := float32(12)
	blockHeight := float32(len(lines))*lineHeight + 2*padding

	rightMargin := ctx.Widget.BodyMargin + ctx.Widget.BodyPadding
	blockWidth := ctx.Width - rightMargin

	backgroundRect := rl.NewRectangle(ctx.X, y, blockWidth, blockHeight)
	rl.DrawRectangleRec(backgroundRect, rl.Color{R: 248, G: 248, B: 248, A: 255})
	rl.DrawRectangleLinesEx(backgroundRect, 1, rl.Color{R: 220, G: 220, B: 220, A: 255})

	currentY := y + padding
	for _, line := range lines {
		ctx.Widget.renderTextWithUnicode(line, ctx.X+padding, currentY, ctx.Widget.Fonts.MonospaceLarge, rl.Color{R: 40, G: 40, B: 40, A: 255})
		currentY += lineHeight
	}

	return RenderResult{
		NextY:  y + blockHeight + 10,
		Height: blockHeight + 20,
	}
}

type CodeRenderHandler struct{}

func (h *CodeRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "code"
}

func (h *CodeRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {

	var content strings.Builder
	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			content.WriteString(child.Content)
		}
	}

	if content.Len() == 0 {
		return RenderResult{NextY: ctx.Y}
	}

	switch node.Context {
	case ContextBlock:
		return h.renderCodeBlock(content.String(), ctx)
	default:
		return h.renderInlineCode(content.String(), ctx)
	}
}

func (h *CodeRenderHandler) renderCodeBlock(content string, ctx RenderContext) RenderResult {
	y := ctx.Y + 10
	lines := strings.Split(content, "\n")
	lineHeight := float32(18)
	padding := float32(12)
	blockHeight := float32(len(lines))*lineHeight + 2*padding

	rightMargin := ctx.Widget.BodyMargin + ctx.Widget.BodyPadding
	blockWidth := ctx.Width - rightMargin

	backgroundRect := rl.NewRectangle(ctx.X, y, blockWidth, blockHeight)
	rl.DrawRectangleRec(backgroundRect, rl.Color{R: 248, G: 248, B: 248, A: 255})
	rl.DrawRectangleLinesEx(backgroundRect, 1, rl.Color{R: 220, G: 220, B: 220, A: 255})

	currentY := y + padding
	for _, line := range lines {
		ctx.Widget.renderTextWithUnicode(line, ctx.X+padding, currentY, ctx.Widget.Fonts.MonospaceLarge, rl.Color{R: 40, G: 40, B: 40, A: 255})
		currentY += lineHeight
	}

	return RenderResult{
		NextY:  y + blockHeight + 10,
		Height: blockHeight + 20,
	}
}

func (h *CodeRenderHandler) renderInlineCode(content string, ctx RenderContext) RenderResult {
	font := ctx.Widget.Fonts.Monospace
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 14
	}

	textSize := ctx.Widget.measureText(font, content, fontSize)
	padding := float32(4)

	renderX := ctx.CurrentX
	if renderX == 0 {
		renderX = ctx.X
	}

	backgroundRect := rl.NewRectangle(renderX-padding, ctx.Y-2, textSize.X+2*padding, textSize.Y+4)
	rl.DrawRectangleRec(backgroundRect, rl.Color{R: 240, G: 240, B: 240, A: 255})
	rl.DrawRectangleLinesEx(backgroundRect, 1, rl.Color{R: 220, G: 220, B: 220, A: 255})

	ctx.Widget.renderTextWithUnicode(content, renderX, ctx.Y, font, rl.Color{R: 40, G: 40, B: 40, A: 255})

	if ctx.CurrentX > 0 {

		return RenderResult{
			NextY:      ctx.Y,
			NextX:      renderX + textSize.X + 2*padding,
			Height:     textSize.Y + 5,
			LineHeight: textSize.Y,
		}
	} else {

		return RenderResult{
			NextY:  ctx.Y + textSize.Y + 5,
			Height: textSize.Y + 5,
		}
	}
}
