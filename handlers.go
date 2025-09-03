package marquee

import (
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// DefinitionListRenderHandler handles dl, dt, dd elements for glossaries and API docs
type DefinitionListRenderHandler struct{}

func (h *DefinitionListRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "dl" || node.Tag == "dt" || node.Tag == "dd"
}

func (h *DefinitionListRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	switch node.Tag {
	case "dl":
		return h.renderDefinitionList(node, ctx)
	case "dt":
		return h.renderDefinitionTerm(node, ctx)
	case "dd":
		return h.renderDefinitionDescription(node, ctx)
	default:
		return RenderResult{NextY: ctx.Y}
	}
}

func (h *DefinitionListRenderHandler) renderDefinitionList(node HTMLNode, ctx RenderContext) RenderResult {
	result := RenderResult{NextY: ctx.Y + 10} // Small top margin

	currentY := result.NextY
	for _, child := range node.Children {
		if child.Tag == "dt" || child.Tag == "dd" {
			childCtx := ctx
			childCtx.Y = currentY
			childCtx.ParentFont = ctx.ParentFont
			childCtx.ParentColor = ctx.ParentColor

			childResult := ctx.Widget.renderer.RenderNode(child, childCtx)
			currentY = childResult.NextY
			result.NextY = childResult.NextY
			result.LinkAreas = append(result.LinkAreas, childResult.LinkAreas...)
		}
	}

	result.NextY += 10 // Bottom margin
	result.Height = result.NextY - ctx.Y
	return result
}

func (h *DefinitionListRenderHandler) renderDefinitionTerm(node HTMLNode, ctx RenderContext) RenderResult {
	// Collect text content
	var content strings.Builder
	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			content.WriteString(child.Content)
		}
	}

	if content.Len() == 0 {
		return RenderResult{NextY: ctx.Y}
	}

	// Render terms in bold, slightly larger font
	font := ctx.Widget.Fonts.Bold
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 18
	}

	text := content.String()
	ctx.Widget.renderTextWithUnicode(text, ctx.X, ctx.Y, font, rl.DarkBlue)

	textSize := ctx.Widget.measureText(font, text, fontSize)
	return RenderResult{
		NextY:  ctx.Y + textSize.Y + 5, // Small gap before definition
		Height: textSize.Y + 5,
	}
}

func (h *DefinitionListRenderHandler) renderDefinitionDescription(node HTMLNode, ctx RenderContext) RenderResult {
	// Indent definition descriptions
	indentedCtx := ctx
	indentedCtx.X = ctx.X + 30 // Indent by 30px
	indentedCtx.Width = ctx.Width - 30
	indentedCtx.CurrentX = indentedCtx.X

	// Use paragraph-style rendering for rich content
	if indentedCtx.ParentFont.BaseSize == 0 {
		indentedCtx.ParentFont = ctx.Widget.Fonts.Regular
	}
	if indentedCtx.ParentColor.R == 0 && indentedCtx.ParentColor.G == 0 && indentedCtx.ParentColor.B == 0 && indentedCtx.ParentColor.A == 0 {
		indentedCtx.ParentColor = rl.Black
	}

	// Build inline segments like paragraphs do
	ph := &ParagraphRenderHandler{}
	segments := h.buildDefinitionSegments(node, indentedCtx)
	result := ph.renderSegmentsWithWrapping(segments, indentedCtx)

	result.NextY += 8 // Gap after definition
	result.Height += 8
	return result
}

func (h *DefinitionListRenderHandler) buildDefinitionSegments(node HTMLNode, ctx RenderContext) []inlineSegment {
	var segments []inlineSegment

	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			segments = append(segments, inlineSegment{
				text:  child.Content,
				font:  ctx.ParentFont,
				color: ctx.ParentColor,
			})
		} else if child.Type == NodeTypeElement && child.Context == ContextInline {
			childSegments := h.getDefinitionSegmentsFromElement(child, ctx)
			segments = append(segments, childSegments...)
		}
	}

	return segments
}

func (h *DefinitionListRenderHandler) getDefinitionSegmentsFromElement(node HTMLNode, ctx RenderContext) []inlineSegment {
	font := ctx.ParentFont
	color := ctx.ParentColor

	// Handle formatting like paragraphs do
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
			nestedSegments := h.getDefinitionSegmentsFromElement(child, nestedCtx)
			segments = append(segments, nestedSegments...)
		}
	}

	return segments
}

// CalloutBoxRenderHandler handles div elements with callout classes for documentation
type CalloutBoxRenderHandler struct{}

func (h *CalloutBoxRenderHandler) CanRender(node HTMLNode) bool {
	if node.Tag != "div" {
		return false
	}

	// Check for callout classes
	class, hasClass := node.Attributes["class"]
	if !hasClass {
		return false
	}

	calloutTypes := []string{"note", "warning", "tip", "info", "danger", "success"}
	for _, calloutType := range calloutTypes {
		if strings.Contains(class, calloutType) {
			return true
		}
	}

	return false
}

func (h *CalloutBoxRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	class, _ := node.Attributes["class"]
	calloutType := h.getCalloutType(class)

	return h.renderCalloutBox(node, ctx, calloutType)
}

func (h *CalloutBoxRenderHandler) getCalloutType(class string) string {
	calloutTypes := []string{"warning", "danger", "success", "info", "tip", "note"}
	for _, calloutType := range calloutTypes {
		if strings.Contains(class, calloutType) {
			return calloutType
		}
	}
	return "note" // default
}

func (h *CalloutBoxRenderHandler) renderCalloutBox(node HTMLNode, ctx RenderContext, calloutType string) RenderResult {
	// Get colors and icon for callout type
	bgColor, borderColor, textColor, icon := h.getCalloutStyle(calloutType)

	// Calculate content first to determine box height
	contentCtx := ctx
	contentCtx.X = ctx.X + 50         // Leave space for icon
	contentCtx.Y = ctx.Y + 15         // Top padding
	contentCtx.Width = ctx.Width - 70 // Account for padding and icon
	contentCtx.ParentFont = ctx.Widget.Fonts.Regular
	contentCtx.ParentColor = textColor
	contentCtx.CurrentX = contentCtx.X

	// Build content segments
	segments := h.buildCalloutSegments(node, contentCtx)

	// Render content to measure height
	ph := &ParagraphRenderHandler{}
	contentResult := ph.renderSegmentsWithWrapping(segments, contentCtx)

	// Calculate total box dimensions
	boxPadding := float32(15)
	boxHeight := contentResult.Height + 2*boxPadding
	boxWidth := ctx.Width - ctx.Widget.BodyMargin - ctx.Widget.BodyPadding

	// Draw box background
	boxRect := rl.NewRectangle(ctx.X, ctx.Y, boxWidth, boxHeight)
	rl.DrawRectangleRec(boxRect, bgColor)

	// Draw left border (thicker for callout effect)
	borderRect := rl.NewRectangle(ctx.X, ctx.Y, 4, boxHeight)
	rl.DrawRectangleRec(borderRect, borderColor)

	// Draw subtle outline
	rl.DrawRectangleLinesEx(boxRect, 1, rl.Color{R: 200, G: 200, B: 200, A: 100})

	// Draw icon
	iconFont := ctx.Widget.Fonts.Regular
	iconY := ctx.Y + boxPadding
	rl.DrawTextEx(iconFont, icon, rl.NewVector2(ctx.X+12, iconY), 18, 1, borderColor)

	// The content was already rendered during measurement, so we need to render it again
	// at the correct position (this is a limitation of our current approach)
	h.renderCalloutContent(segments, contentCtx, ctx.Widget)

	return RenderResult{
		NextY:     ctx.Y + boxHeight + 15, // Bottom margin
		Height:    boxHeight + 15,
		LinkAreas: contentResult.LinkAreas,
	}
}

func (h *CalloutBoxRenderHandler) getCalloutStyle(calloutType string) (rl.Color, rl.Color, rl.Color, string) {
	switch calloutType {
	case "warning":
		return rl.Color{R: 255, G: 248, B: 220, A: 255}, // Light yellow background
			rl.Color{R: 255, G: 193, B: 7, A: 255}, // Orange border
			rl.Color{R: 133, G: 77, B: 14, A: 255}, // Dark orange text
			"⚠️"

	case "danger":
		return rl.Color{R: 253, G: 237, B: 237, A: 255}, // Light red background
			rl.Color{R: 220, G: 38, B: 127, A: 255}, // Red border
			rl.Color{R: 114, G: 28, B: 36, A: 255}, // Dark red text
			"🚫"

	case "success":
		return rl.Color{R: 230, G: 245, B: 233, A: 255}, // Light green background
			rl.Color{R: 40, G: 167, B: 69, A: 255}, // Green border
			rl.Color{R: 21, G: 87, B: 36, A: 255}, // Dark green text
			"✅"

	case "info":
		return rl.Color{R: 217, G: 237, B: 247, A: 255}, // Light blue background
			rl.Color{R: 52, G: 144, B: 220, A: 255}, // Blue border
			rl.Color{R: 12, G: 84, B: 96, A: 255}, // Dark blue text
			"ℹ️"

	case "tip":
		return rl.Color{R: 230, G: 245, B: 233, A: 255}, // Light green background
			rl.Color{R: 40, G: 167, B: 69, A: 255}, // Green border
			rl.Color{R: 21, G: 87, B: 36, A: 255}, // Dark green text
			"💡"

	default: // "note"
		return rl.Color{R: 248, G: 249, B: 250, A: 255}, // Light gray background
			rl.Color{R: 108, G: 117, B: 125, A: 255}, // Gray border
			rl.Color{R: 33, G: 37, B: 41, A: 255}, // Dark gray text
			"📝"
	}
}

func (h *CalloutBoxRenderHandler) buildCalloutSegments(node HTMLNode, ctx RenderContext) []inlineSegment {
	var segments []inlineSegment

	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			segments = append(segments, inlineSegment{
				text:  child.Content,
				font:  ctx.ParentFont,
				color: ctx.ParentColor,
			})
		} else if child.Type == NodeTypeElement && child.Context == ContextInline {
			childSegments := h.getCalloutSegmentsFromElement(child, ctx)
			segments = append(segments, childSegments...)
		}
	}

	return segments
}

func (h *CalloutBoxRenderHandler) getCalloutSegmentsFromElement(node HTMLNode, ctx RenderContext) []inlineSegment {
	font := ctx.ParentFont
	color := ctx.ParentColor

	// Handle formatting
	if node.Tag == "span" {
		if style, exists := node.Attributes["style"]; exists {
			if strings.Contains(style, "font-weight: bold") {
				font = ctx.Widget.Fonts.Bold
				// Keep callout text color, don't override with blue
			}
			if strings.Contains(style, "font-style: italic") {
				font = ctx.Widget.Fonts.Italic
				// Keep callout text color, don't override with green
			}
		}
	} else if node.Tag == "a" {
		color = rl.Blue // Links can still be blue in callouts
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
			nestedSegments := h.getCalloutSegmentsFromElement(child, nestedCtx)
			segments = append(segments, nestedSegments...)
		}
	}

	return segments
}

func (h *CalloutBoxRenderHandler) renderCalloutContent(segments []inlineSegment, ctx RenderContext, widget *HTMLWidget) {
	currentY := ctx.Y
	lineHeight := float32(20)
	currentLineSegments := []inlineSegment{}
	currentLineWidth := float32(0)

	rightMargin := widget.BodyMargin + widget.BodyPadding

	for _, segment := range segments {
		ph := &ParagraphRenderHandler{}
		words := ph.intelligentWordSplit(segment.text)

		for _, word := range words {
			wordSegment := inlineSegment{
				text:  word,
				font:  segment.font,
				color: segment.color,
				href:  segment.href,
			}

			wordWidth := widget.measureTextWidth(segment.font, word, float32(segment.font.BaseSize))

			availableWidth := ctx.Width - rightMargin
			if currentLineWidth+wordWidth > availableWidth && len(currentLineSegments) > 0 {
				h.renderCalloutInlineSegments(currentLineSegments, ctx.X, currentY, widget)

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
					currentLineWidth += widget.measureTextWidth(segment.font, " ", float32(segment.font.BaseSize))
				}

				currentLineSegments = append(currentLineSegments, wordSegment)
				currentLineWidth += wordWidth
			}
		}
	}

	if len(currentLineSegments) > 0 {
		h.renderCalloutInlineSegments(currentLineSegments, ctx.X, currentY, widget)
	}
}

func (h *CalloutBoxRenderHandler) renderCalloutInlineSegments(segments []inlineSegment, x, y float32, widget *HTMLWidget) {
	currentX := x

	for _, segment := range segments {
		widget.renderTextWithUnicode(segment.text, currentX, y, segment.font, segment.color)
		segmentWidth := widget.measureTextWidth(segment.font, segment.text, float32(segment.font.BaseSize))

		// Note: We're not handling link areas here for simplicity
		// Could be added if needed by collecting them and returning

		currentX += segmentWidth
	}
}
