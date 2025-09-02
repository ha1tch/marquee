package marquee

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// BUG #9 FIX: Enhanced Unicode support for international content
var essentialCodepoints []rune

func init() {
	// ASCII printable range (32-126)
	for i := rune(32); i <= 126; i++ {
		essentialCodepoints = append(essentialCodepoints, i)
	}

	// BUG #9 FIX: Extended Unicode support for international content
	// Latin-1 Supplement (À-ÿ) - covers French, German, Spanish, Italian
	for i := rune(0x00C0); i <= 0x00FF; i++ {
		essentialCodepoints = append(essentialCodepoints, i)
	}
	
	// Latin Extended-A (Ā-ſ) - covers Eastern European languages
	for i := rune(0x0100); i <= 0x017F; i++ {
		essentialCodepoints = append(essentialCodepoints, i)
	}

	// Essential Unicode punctuation and symbols
	essentialUnicode := []rune{
		0x2022, // • BULLET
		0x25CF, // ● BLACK CIRCLE
		0x2013, // – EN DASH
		0x2014, // — EM DASH
		0x201C, // " LEFT DOUBLE QUOTATION MARK
		0x201D, // " RIGHT DOUBLE QUOTATION MARK
		0x2018, // ' LEFT SINGLE QUOTATION MARK
		0x2019, // ' RIGHT SINGLE QUOTATION MARK
		0x2026, // … HORIZONTAL ELLIPSIS
		0x00A0, // Non-breaking space
		0x00AB, // « LEFT-POINTING DOUBLE ANGLE QUOTATION MARK
		0x00BB, // » RIGHT-POINTING DOUBLE ANGLE QUOTATION MARK
	}
	essentialCodepoints = append(essentialCodepoints, essentialUnicode...)
}

// DOCUMENT MODEL DATA STRUCTURES

// NodeType defines the fundamental type of HTML content
type NodeType int

const (
	NodeTypeText NodeType = iota
	NodeTypeElement
	NodeTypeDocument
)

// NodeContext defines rendering context
type NodeContext int

const (
	ContextBlock NodeContext = iota
	ContextInline
	ContextRoot
)

// HTMLNode represents a single node in the document tree
type HTMLNode struct {
	Type       NodeType
	Tag        string
	Content    string
	Attributes map[string]string
	Children   []HTMLNode
	Context    NodeContext
	Parent     *HTMLNode
}

// HTMLDocument represents the complete parsed document
type HTMLDocument struct {
	Root     HTMLNode
	Metadata DocumentMetadata
}

// DocumentMetadata holds invisible document information
type DocumentMetadata struct {
	Title       string
	Scripts     []ScriptInfo
	StyleSheets []StyleInfo
	MetaTags    []MetaInfo
	DocType     string
}

// Metadata structures for future use
type ScriptInfo struct {
	Src     string
	Content string
	Type    string
}

type StyleInfo struct {
	Href    string
	Content string
	Media   string
}

type MetaInfo struct {
	Name    string
	Content string
	Charset string
}

// STATE MACHINE PARSER - FIXED FOR BUG #19

// ParserState tracks where we are in the HTML
type ParserState int

const (
	StateText ParserState = iota
	StateTagOpen
	StateTagName
	StateAttributes
	StateAttributeName
	StateAttributeValue
	StateAttributeQuoted
	StateTagClose
	StateEndTag
	StateComment
)

// StateMachineParser builds HTMLDocument using state transitions
type StateMachineParser struct {
	input        []rune
	position     int
	state        ParserState
	nodeStack    []NodeStackEntry
	textBuffer   strings.Builder
	tagBuffer    strings.Builder
	attrName     string
	attrValue    strings.Builder
	currentAttrs map[string]string
	quoteChar    rune
	// BUG #19 FIX: Add parser validation and safety
	maxDepth     int
	currentDepth int
	maxLength    int
}

// NodeStackEntry represents an entry on the parser stack
type NodeStackEntry struct {
	Node        *HTMLNode
	OriginalTag string
}

// NewStateMachineParser creates a new state machine parser
func NewStateMachineParser() *StateMachineParser {
	return &StateMachineParser{
		currentAttrs: make(map[string]string),
		maxDepth:     50,   // Prevent stack overflow from deeply nested HTML
		maxLength:    1000000, // Prevent memory exhaustion from huge documents
	}
}

// BUG #19 FIX: Add parser reset method
func (p *StateMachineParser) Reset() {
	p.input = nil
	p.position = 0
	p.state = StateText
	p.nodeStack = nil
	p.textBuffer.Reset()
	p.tagBuffer.Reset()
	p.attrName = ""
	p.attrValue.Reset()
	p.currentAttrs = make(map[string]string)
	p.quoteChar = 0
	p.currentDepth = 0
}

// Parse converts HTML string to HTMLDocument using state machine
func (p *StateMachineParser) Parse(html string) HTMLDocument {
	// BUG #19 FIX: Reset parser state and validate input
	p.Reset()
	
	if len(html) > p.maxLength {
		html = html[:p.maxLength] // Truncate extremely large documents
	}
	
	// Initialize parser state
	p.input = []rune(strings.TrimSpace(html))
	p.position = 0
	p.state = StateText

	// Create root document node
	root := &HTMLNode{
		Type:       NodeTypeDocument,
		Context:    ContextRoot,
		Attributes: make(map[string]string),
		Children:   make([]HTMLNode, 0),
	}
	p.nodeStack = []NodeStackEntry{{Node: root, OriginalTag: "document"}}

	// Process each character with bounds checking
	for p.position < len(p.input) {
		char := p.input[p.position]

		// BUG #19 FIX: Add safety checks
		if p.currentDepth > p.maxDepth {
			break // Prevent stack overflow
		}

		switch p.state {
		case StateText:
			p.handleTextState(char)
		case StateTagOpen:
			p.handleTagOpenState(char)
		case StateTagName:
			p.handleTagNameState(char)
		case StateAttributes:
			p.handleAttributesState(char)
		case StateAttributeName:
			p.handleAttributeNameState(char)
		case StateAttributeValue:
			p.handleAttributeValueState(char)
		case StateAttributeQuoted:
			p.handleAttributeQuotedState(char)
		case StateTagClose:
			p.handleTagCloseState(char)
		case StateEndTag:
			p.handleEndTagState(char)
		case StateComment:
			p.handleCommentState(char)
		}

		p.position++
		
		// BUG #19 FIX: Prevent infinite loops
		if p.position > len(p.input) {
			break
		}
	}

	// Flush any remaining text
	if p.textBuffer.Len() > 0 {
		p.addTextNode(p.textBuffer.String())
	}

	// BUG #19 FIX: Close any unclosed tags
	for len(p.nodeStack) > 1 {
		p.nodeStack = p.nodeStack[:len(p.nodeStack)-1]
	}

	return HTMLDocument{Root: *root}
}

// State transition handlers - Enhanced with safety checks
func (p *StateMachineParser) handleTextState(char rune) {
	if char == '<' {
		// Flush current text content
		if p.textBuffer.Len() > 0 {
			p.addTextNode(p.textBuffer.String())
			p.textBuffer.Reset()
		}
		p.state = StateTagOpen
	} else {
		p.textBuffer.WriteRune(char)
	}
}

func (p *StateMachineParser) handleTagOpenState(char rune) {
	if char == '/' {
		p.state = StateEndTag
		p.tagBuffer.Reset()
	} else if char == '!' {
		p.state = StateComment
	} else if char == ' ' || char == '\t' || char == '\n' {
		// Skip whitespace after <
	} else {
		// Start reading tag name
		p.tagBuffer.Reset()
		p.tagBuffer.WriteRune(char)
		p.state = StateTagName
		p.currentAttrs = make(map[string]string)
	}
}

func (p *StateMachineParser) handleTagNameState(char rune) {
	if char == ' ' || char == '\t' || char == '\n' {
		p.state = StateAttributes
	} else if char == '>' {
		p.finishOpenTag()
		p.state = StateText
	} else if char == '/' {
		p.state = StateTagClose
	} else {
		p.tagBuffer.WriteRune(char)
	}
}

func (p *StateMachineParser) handleAttributesState(char rune) {
	if char == '>' {
		p.finishOpenTag()
		p.state = StateText
	} else if char == '/' {
		p.state = StateTagClose
	} else if char != ' ' && char != '\t' && char != '\n' {
		p.attrName = string(char)
		p.state = StateAttributeName
	}
}

func (p *StateMachineParser) handleAttributeNameState(char rune) {
	if char == '=' {
		p.state = StateAttributeValue
		p.attrValue.Reset()
	} else if char == ' ' || char == '\t' || char == '\n' {
		p.currentAttrs[p.attrName] = p.attrName
		p.state = StateAttributes
	} else if char == '>' {
		p.currentAttrs[p.attrName] = p.attrName
		p.finishOpenTag()
		p.state = StateText
	} else {
		p.attrName += string(char)
	}
}

func (p *StateMachineParser) handleAttributeValueState(char rune) {
	if char == '"' || char == '\'' {
		p.quoteChar = char
		p.state = StateAttributeQuoted
	} else if char == ' ' || char == '\t' || char == '\n' {
		p.currentAttrs[p.attrName] = p.attrValue.String()
		p.state = StateAttributes
	} else if char == '>' {
		p.currentAttrs[p.attrName] = p.attrValue.String()
		p.finishOpenTag()
		p.state = StateText
	} else {
		p.attrValue.WriteRune(char)
	}
}

func (p *StateMachineParser) handleAttributeQuotedState(char rune) {
	if char == p.quoteChar {
		p.currentAttrs[p.attrName] = p.attrValue.String()
		p.state = StateAttributes
	} else {
		p.attrValue.WriteRune(char)
	}
}

func (p *StateMachineParser) handleTagCloseState(char rune) {
	if char == '>' {
		p.finishSelfClosingTag()
		p.state = StateText
	}
}

func (p *StateMachineParser) handleEndTagState(char rune) {
	if char == '>' {
		p.finishEndTag()
		p.state = StateText
	} else if char != ' ' && char != '\t' && char != '\n' {
		p.tagBuffer.WriteRune(char)
	}
}

// BUG #19 FIX: Enhanced comment handling with safety
func (p *StateMachineParser) handleCommentState(char rune) {
	// Look for --> to end comment, with bounds checking
	if char == '>' && p.position >= 2 {
		if p.position-1 < len(p.input) && p.position-2 < len(p.input) &&
			p.input[p.position-1] == '-' && p.input[p.position-2] == '-' {
			p.state = StateText
		}
	}
	// BUG #19 FIX: Prevent infinite comment parsing
	if p.position > 1000 { // Arbitrary limit to prevent infinite comment parsing
		p.state = StateText
	}
}

// Helper functions for state machine
func (p *StateMachineParser) addTextNode(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}

	if len(p.nodeStack) == 0 {
		return // BUG #19 FIX: Safety check
	}

	parent := p.nodeStack[len(p.nodeStack)-1].Node
	textNode := HTMLNode{
		Type:    NodeTypeText,
		Content: content,
		Context: ContextInline,
		Parent:  parent,
	}

	parent.Children = append(parent.Children, textNode)
}

func (p *StateMachineParser) finishOpenTag() {
	tagName := strings.ToLower(p.tagBuffer.String())

	// BUG #19 FIX: Validate tag name
	if tagName == "" {
		p.tagBuffer.Reset()
		p.currentAttrs = make(map[string]string)
		return
	}

	// Create new element node
	node := HTMLNode{
		Type:       NodeTypeElement,
		Tag:        tagName,
		Attributes: make(map[string]string),
		Children:   make([]HTMLNode, 0),
	}

	// Copy attributes
	for k, v := range p.currentAttrs {
		node.Attributes[k] = v
	}

	// Determine context
	if len(p.nodeStack) == 0 {
		return // BUG #19 FIX: Safety check
	}
	
	parent := p.nodeStack[len(p.nodeStack)-1].Node
	node.Context = p.determineContext(tagName, parent)
	node.Parent = parent

	// Normalize formatting tags to spans
	originalTag := tagName
	node = *p.normalizeElement(&node)

	// Add to parent
	parent.Children = append(parent.Children, node)

	// If this is a container element, push onto stack
	if p.isContainerElement(originalTag) {
		// BUG #19 FIX: Check depth before pushing
		if p.currentDepth < p.maxDepth {
			childIndex := len(parent.Children) - 1
			childNode := &parent.Children[childIndex]

			stackEntry := NodeStackEntry{
				Node:        childNode,
				OriginalTag: originalTag,
			}
			p.nodeStack = append(p.nodeStack, stackEntry)
			p.currentDepth++
		}
	}

	p.tagBuffer.Reset()
	p.currentAttrs = make(map[string]string)
}

func (p *StateMachineParser) finishSelfClosingTag() {
	tagName := strings.ToLower(p.tagBuffer.String())

	// BUG #19 FIX: Validate tag name and stack
	if tagName == "" || len(p.nodeStack) == 0 {
		p.tagBuffer.Reset()
		p.currentAttrs = make(map[string]string)
		return
	}

	parent := p.nodeStack[len(p.nodeStack)-1].Node
	node := HTMLNode{
		Type:       NodeTypeElement,
		Tag:        tagName,
		Attributes: make(map[string]string),
		Context:    p.determineContext(tagName, parent),
		Parent:     parent,
	}

	for k, v := range p.currentAttrs {
		node.Attributes[k] = v
	}

	node = *p.normalizeElement(&node)
	parent.Children = append(parent.Children, node)

	p.tagBuffer.Reset()
	p.currentAttrs = make(map[string]string)
}

func (p *StateMachineParser) finishEndTag() {
	tagName := strings.ToLower(p.tagBuffer.String())

	// BUG #19 FIX: Enhanced end tag validation
	if tagName == "" || len(p.nodeStack) <= 1 {
		p.tagBuffer.Reset()
		return
	}

	// Find matching open tag in stack (handle malformed HTML)
	for i := len(p.nodeStack) - 1; i > 0; i-- {
		current := p.nodeStack[i]

		if current.Node.Tag == tagName || current.OriginalTag == tagName {
			// Close all tags up to this point
			p.nodeStack = p.nodeStack[:i]
			p.currentDepth = len(p.nodeStack) - 1
			break
		}
	}

	p.tagBuffer.Reset()
}

func (p *StateMachineParser) determineContext(tagName string, parent *HTMLNode) NodeContext {
	blockTags := map[string]bool{
		"p": true, "div": true, "h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true, "ul": true, "ol": true,
		"li": true, "pre": true, "hr": true,
	}

	if blockTags[tagName] {
		return ContextBlock
	}

	// CRITICAL FIX: Children of both paragraphs AND list items should be inline
	if parent.Tag == "p" || parent.Tag == "li" {
		return ContextInline
	}

	if parent.Context == ContextRoot {
		return ContextBlock
	}

	return parent.Context
}

func (p *StateMachineParser) isContainerElement(tagName string) bool {
	containers := map[string]bool{
		"p": true, "div": true, "ul": true, "ol": true, "li": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"a": true, "b": true, "i": true, "span": true, "pre": true, "code": true,
	}
	return containers[tagName]
}

// normalizeElement converts formatting tags to spans with style attributes
func (p *StateMachineParser) normalizeElement(node *HTMLNode) *HTMLNode {
	switch node.Tag {
	case "b":
		node.Tag = "span"
		node.Attributes["style"] = "font-weight: bold"
	case "i":
		node.Tag = "span"
		node.Attributes["style"] = "font-style: italic"
	case "strong":
		node.Tag = "span"
		node.Attributes["style"] = "font-weight: bold"
	case "em":
		node.Tag = "span"
		node.Attributes["style"] = "font-style: italic"
	}
	return node
}

// CONTEXT-AWARE RENDERER

// RenderContext provides information needed for rendering
type RenderContext struct {
	X, Y, Width float32
	ParentFont  rl.Font
	ParentColor rl.Color
	Indent      int
	LineHeight  float32
	Widget      *HTMLWidget // Access to widget resources
	// BUG #4 FIX: Add X position tracking for inline elements
	CurrentX    float32
	MaxLineHeight float32 // Track maximum height in current line
}

// RenderResult contains rendering output
type RenderResult struct {
	NextY     float32
	LinkAreas []LinkArea
	Height    float32
	// BUG #4 FIX: Add X advancement for inline elements
	NextX     float32
	LineHeight float32
}

// RenderHandler handles rendering for specific element types
type RenderHandler interface {
	CanRender(node HTMLNode) bool
	Render(node HTMLNode, ctx RenderContext) RenderResult
}

// HTMLRenderer handles all rendering with context awareness
type HTMLRenderer struct {
	handlers map[string]RenderHandler
}

// NewHTMLRenderer creates a new context-aware renderer
func NewHTMLRenderer() *HTMLRenderer {
	r := &HTMLRenderer{
		handlers: make(map[string]RenderHandler),
	}

	// Register all render handlers
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
	// DON'T register li - let parent lists handle them
	r.RegisterHandler("hr", &HRRenderHandler{})
	r.RegisterHandler("br", &BreakRenderHandler{})
	r.RegisterHandler("pre", &PreRenderHandler{})
	r.RegisterHandler("code", &CodeRenderHandler{})

	return r
}

// RegisterHandler adds a render handler for a specific tag
func (r *HTMLRenderer) RegisterHandler(tag string, handler RenderHandler) {
	r.handlers[tag] = handler
}

// RenderNode renders a single node using the appropriate handler
func (r *HTMLRenderer) RenderNode(node HTMLNode, ctx RenderContext) RenderResult {
	if handler, exists := r.handlers[node.Tag]; exists && handler.CanRender(node) {
		return handler.Render(node, ctx)
	}

	// Fallback: render as text
	return r.handlers["text"].Render(node, ctx)
}

// RenderDocument renders the entire document
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

// CONTEXT-AWARE RENDER HANDLERS

// TextRenderHandler handles text nodes and fallback rendering
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

// BUG #4 FIX: Enhanced SpanRenderHandler with proper inline positioning
type SpanRenderHandler struct{}

func (h *SpanRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "span"
}

func (h *SpanRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	// BUG #3 FIX: Enhanced font and color detection from style
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

	// BUG #4 FIX: Handle inline vs block context properly
	if node.Context == ContextInline {
		// For inline elements, collect all text content
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

		// Render inline text at current X position
		ctx.Widget.renderTextWithUnicode(text, ctx.CurrentX, ctx.Y, font, color)
		
		// Calculate text width to advance X position
		textWidth := ctx.Widget.measureTextWidth(font, text, fontSize)

		return RenderResult{
			NextY:      ctx.Y,           // Don't advance Y for inline
			NextX:      ctx.CurrentX + textWidth, // Advance X for next inline element
			Height:     fontSize,
			LineHeight: fontSize,
		}
	}

	// For block spans, render as block
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

// BUG #5 FIX: Enhanced LinkRenderHandler with correct coordinate system
type LinkRenderHandler struct{}

func (h *LinkRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "a"
}

func (h *LinkRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	href, _ := node.Attributes["href"]

	// Collect text content from children
	var content strings.Builder
	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			content.WriteString(child.Content)
		}
	}

	if content.Len() == 0 {
		return RenderResult{NextY: ctx.Y, NextX: ctx.CurrentX}
	}

	// BUG #1 FIX: Use parent font instead of hardcoded regular font
	font := ctx.ParentFont
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16
	}

	text := content.String()
	textSize := ctx.Widget.measureText(font, text, fontSize)

	// BUG #1 FIX: Links should be blue but inherit parent font style
	color := rl.Blue

	// BUG #5 FIX: Create link area in screen coordinates
	bounds := rl.NewRectangle(ctx.CurrentX, ctx.Y, textSize.X, textSize.Y)
	linkArea := LinkArea{Bounds: bounds, URL: href}

	// BUG #4 FIX: Handle inline vs block links
	if node.Context == ContextInline {
		// Render inline link
		ctx.Widget.renderTextWithUnicode(text, ctx.CurrentX, ctx.Y, font, color)

		// Draw underline
		rl.DrawLineEx(
			rl.NewVector2(ctx.CurrentX, ctx.Y+textSize.Y),
			rl.NewVector2(ctx.CurrentX+textSize.X, ctx.Y+textSize.Y),
			1, color)

		return RenderResult{
			NextY:     ctx.Y,                    // Don't advance Y for inline
			NextX:     ctx.CurrentX + textSize.X, // Advance X position
			LinkAreas: []LinkArea{linkArea},
			Height:    textSize.Y,
			LineHeight: textSize.Y,
		}
	} else {
		// Render as block link
		ctx.Widget.renderTextWithUnicode(text, ctx.X, ctx.Y, font, color)

		// Draw underline
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

// BUG #22 FIX: Enhanced HeadingRenderHandler with proper spacing hierarchy
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

	// BUG #22 FIX: Progressive spacing based on heading level
	spacingBefore := float32([]int{25, 20, 18, 15, 12, 10}[level-1])
	spacingAfter := float32([]int{15, 12, 10, 8, 6, 5}[level-1])
	
	y := ctx.Y + spacingBefore

	// Collect text content
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

// BUG #2 & #4 FIX: Enhanced ParagraphRenderHandler with proper font context propagation
type ParagraphRenderHandler struct{}

func (h *ParagraphRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "p"
}

func (h *ParagraphRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	// BUG #2 FIX: Ensure paragraph gets proper font context
	if ctx.ParentFont.BaseSize == 0 {
		ctx.ParentFont = ctx.Widget.Fonts.Regular
	}
	if ctx.ParentColor.R == 0 && ctx.ParentColor.G == 0 && ctx.ParentColor.B == 0 && ctx.ParentColor.A == 0 {
		ctx.ParentColor = rl.Black
	}

	// Build inline segments with proper font context
	segments := h.buildInlineSegments(node, ctx)
	
	// Render segments with word wrapping and X position tracking
	return h.renderSegmentsWithWrapping(segments, ctx)
}

type inlineSegment struct {
	text  string
	font  rl.Font
	color rl.Color
	href  string // For links
}

// BUG #2 FIX: Build segments with proper font inheritance
func (h *ParagraphRenderHandler) buildInlineSegments(node HTMLNode, ctx RenderContext) []inlineSegment {
	var segments []inlineSegment

	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			segments = append(segments, inlineSegment{
				text:  child.Content,
				font:  ctx.ParentFont, // BUG #2 FIX: Use parent font instead of default
				color: ctx.ParentColor,
			})
		} else if child.Type == NodeTypeElement && child.Context == ContextInline {
			childSegments := h.getSegmentsFromElement(child, ctx)
			segments = append(segments, childSegments...)
		}
	}

	return segments
}

// BUG #3 FIX: Enhanced font resolution with proper style detection
func (h *ParagraphRenderHandler) getSegmentsFromElement(node HTMLNode, ctx RenderContext) []inlineSegment {
	font := ctx.ParentFont // Start with parent font
	color := ctx.ParentColor

	// BUG #3 FIX: Enhanced style detection for spans (normalized from b/i tags)
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
		// BUG #1 FIX: Links inherit parent font but use blue color
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
			// BUG #3 FIX: Handle nested inline elements recursively
			nestedCtx := ctx
			nestedCtx.ParentFont = font
			nestedCtx.ParentColor = color
			nestedSegments := h.getSegmentsFromElement(child, nestedCtx)
			segments = append(segments, nestedSegments...)
		}
	}

	return segments
}

// BUG #4 & #6 FIX: Enhanced segment rendering with proper margin calculations
func (h *ParagraphRenderHandler) renderSegmentsWithWrapping(segments []inlineSegment, ctx RenderContext) RenderResult {
	result := RenderResult{NextY: ctx.Y}
	
	currentY := ctx.Y
	lineHeight := float32(20)
	currentLineSegments := []inlineSegment{}
	currentLineWidth := float32(0)
	
	// BUG #6 FIX: Use proper margin calculations instead of hardcoded values
	rightMargin := ctx.Widget.BodyMargin + ctx.Widget.BodyPadding

	for _, segment := range segments {
		words := strings.Fields(segment.text)

		for _, word := range words {
			wordSegment := inlineSegment{
				text:  word,
				font:  segment.font,
				color: segment.color,
				href:  segment.href,
			}

			wordWidth := ctx.Widget.measureTextWidth(segment.font, word, float32(segment.font.BaseSize))
			
			// BUG #6 FIX: Check against actual available width with proper margins
			availableWidth := ctx.Width - rightMargin
			if currentLineWidth + wordWidth > availableWidth && len(currentLineSegments) > 0 {
				// Render current line
				h.renderInlineSegments(currentLineSegments, ctx.X, currentY, ctx.Widget, &result)
				
				// Move to next line
				currentY += lineHeight
				currentLineSegments = []inlineSegment{wordSegment}
				currentLineWidth = wordWidth
			} else {
				// Add space before word if not first segment on line
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

	// Render remaining line
	if len(currentLineSegments) > 0 {
		h.renderInlineSegments(currentLineSegments, ctx.X, currentY, ctx.Widget, &result)
		currentY += lineHeight
	}

	result.NextY = currentY + 5
	result.Height = result.NextY - ctx.Y
	return result
}

// BUG #4 & #5 FIX: Render inline segments with proper positioning and link areas
func (h *ParagraphRenderHandler) renderInlineSegments(segments []inlineSegment, x, y float32, widget *HTMLWidget, result *RenderResult) {
	currentX := x
	
	for _, segment := range segments {
		widget.renderTextWithUnicode(segment.text, currentX, y, segment.font, segment.color)
		
		segmentWidth := widget.measureTextWidth(segment.font, segment.text, float32(segment.font.BaseSize))
		
		// BUG #5 FIX: Create link areas in correct screen coordinates
		if segment.href != "" {
			fontSize := float32(segment.font.BaseSize)
			if fontSize == 0 {
				fontSize = 16
			}
			
			// Create link area in screen coordinates (not document coordinates)
			bounds := rl.NewRectangle(currentX, y, segmentWidth, fontSize)
			linkArea := LinkArea{Bounds: bounds, URL: segment.href}
			result.LinkAreas = append(result.LinkAreas, linkArea)
			
			// Draw underline for links
			rl.DrawLineEx(
				rl.NewVector2(currentX, y+fontSize),
				rl.NewVector2(currentX+segmentWidth, y+fontSize),
				1, segment.color)
		}
		
		currentX += segmentWidth
	}
}

// BUG #11 & #21 FIX: Enhanced ListRenderHandler with proper indentation
type ListRenderHandler struct{}

func (h *ListRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "ul" || node.Tag == "ol" || node.Tag == "li"
}

func (h *ListRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	if node.Tag == "li" {
		// Handle individual list items
		return h.renderListItem(node, ctx, "ul", 0) // Default to unordered, parent should set context
	}

	// Handle list containers (ul/ol)
	result := RenderResult{NextY: ctx.Y + 10}

	currentY := result.NextY
	for i, child := range node.Children {
		if child.Tag == "li" {
			// BUG #21 FIX: Proper nested indentation calculation
			childCtx := ctx
			baseIndent := float32(25) // Base indentation for first level
			nestedIndent := float32(ctx.Indent * 20) // Additional for nested levels
			childCtx.X = ctx.X + baseIndent + nestedIndent
			childCtx.Y = currentY
			childCtx.Width = ctx.Width - baseIndent - nestedIndent - ctx.Widget.BodyMargin
			childCtx.Indent = ctx.Indent + 1
			childCtx.ParentFont = ctx.ParentFont
			childCtx.ParentColor = ctx.ParentColor
			
			// Create rendering context with list info
			listItemResult := h.renderListItem(child, childCtx, node.Tag, i)
			currentY = listItemResult.NextY
			result.NextY = listItemResult.NextY
			result.LinkAreas = append(result.LinkAreas, listItemResult.LinkAreas...)
		}
	}

	result.Height = result.NextY - ctx.Y
	return result
}

// BUG #11 FIX: Render list item without mutating document tree
func (h *ListRenderHandler) renderListItem(node HTMLNode, ctx RenderContext, listType string, index int) RenderResult {
	// Draw bullet or number with enhanced visibility
	bulletFont := ctx.Widget.Fonts.Regular
	if listType == "ol" {
		marker := fmt.Sprintf("%d.", index+1)
		rl.DrawTextEx(bulletFont, marker, rl.NewVector2(ctx.X-20, ctx.Y), 16, 1, rl.Black)
	} else {
		// Use a larger, more visible bullet
		bulletRune := rune(0x2022) // • BULLET
		bulletStr := string(bulletRune)
		rl.DrawTextEx(bulletFont, bulletStr, rl.NewVector2(ctx.X-15, ctx.Y), 18, 1, rl.Black)
	}

	// Render content with proper font context (same as paragraphs)
	contentCtx := ctx
	contentCtx.CurrentX = ctx.X
	// Preserve font context from parent
	if contentCtx.ParentFont.BaseSize == 0 {
		contentCtx.ParentFont = ctx.Widget.Fonts.Regular
	}
	if contentCtx.ParentColor.R == 0 && contentCtx.ParentColor.G == 0 && contentCtx.ParentColor.B == 0 && contentCtx.ParentColor.A == 0 {
		contentCtx.ParentColor = rl.Black
	}

	return h.renderListItemContent(node, contentCtx)
}

// BUG #2 & #3 FIX: Render list item content with proper font inheritance
func (h *ListRenderHandler) renderListItemContent(node HTMLNode, ctx RenderContext) RenderResult {
	// Use the same segment-based approach as paragraphs
	segments := h.buildListItemSegments(node, ctx)
	return h.renderListItemSegments(segments, ctx)
}

// BUG #2 & #3 FIX: Build segments with proper font detection
func (h *ListRenderHandler) buildListItemSegments(node HTMLNode, ctx RenderContext) []inlineSegment {
	var segments []inlineSegment

	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			segments = append(segments, inlineSegment{
				text:  child.Content,
				font:  ctx.ParentFont, // BUG #2 FIX: Use widget font
				color: ctx.ParentColor,
			})
		} else if child.Type == NodeTypeElement && child.Context == ContextInline {
			childSegments := h.getListItemSegmentsFromElement(child, ctx)
			segments = append(segments, childSegments...)
		}
	}

	return segments
}

// BUG #1 & #3 FIX: Enhanced font detection for list item elements
func (h *ListRenderHandler) getListItemSegmentsFromElement(node HTMLNode, ctx RenderContext) []inlineSegment {
	font := ctx.ParentFont
	color := ctx.ParentColor

	// BUG #3 FIX: Proper style detection for normalized elements
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
		// BUG #1 FIX: Links inherit parent font but use blue color
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
			// Handle nested inline elements
			nestedCtx := ctx
			nestedCtx.ParentFont = font
			nestedCtx.ParentColor = color
			nestedSegments := h.getListItemSegmentsFromElement(child, nestedCtx)
			segments = append(segments, nestedSegments...)
		}
	}

	return segments
}

// BUG #4 & #6 FIX: Enhanced list item rendering with proper margin calculations
func (h *ListRenderHandler) renderListItemSegments(segments []inlineSegment, ctx RenderContext) RenderResult {
	result := RenderResult{NextY: ctx.Y}
	
	currentY := ctx.Y
	lineHeight := float32(20)
	currentLineSegments := []inlineSegment{}
	currentLineWidth := float32(0)
	
	// BUG #6 FIX: Use proper margin calculations instead of hardcoded values
	rightMargin := ctx.Widget.BodyMargin + ctx.Widget.BodyPadding

	for _, segment := range segments {
		words := strings.Fields(segment.text)

		for _, word := range words {
			wordSegment := inlineSegment{
				text:  word,
				font:  segment.font,
				color: segment.color,
				href:  segment.href,
			}

			wordWidth := ctx.Widget.measureTextWidth(segment.font, word, float32(segment.font.BaseSize))
			
			// BUG #6 FIX: Check against actual available width with proper margins
			availableWidth := ctx.Width - rightMargin
			if currentLineWidth + wordWidth > availableWidth && len(currentLineSegments) > 0 {
				// Render current line
				h.renderListItemInlineSegments(currentLineSegments, ctx.X, currentY, ctx.Widget, &result)
				
				// Move to next line
				currentY += lineHeight
				currentLineSegments = []inlineSegment{wordSegment}
				currentLineWidth = wordWidth
			} else {
				// Add space before word if not first segment on line
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

	// Render remaining line
	if len(currentLineSegments) > 0 {
		h.renderListItemInlineSegments(currentLineSegments, ctx.X, currentY, ctx.Widget, &result)
		currentY += lineHeight
	}

	result.NextY = currentY + 5
	result.Height = result.NextY - ctx.Y
	return result
}

// BUG #1 & #5 FIX: Render segments with proper formatting and link handling
func (h *ListRenderHandler) renderListItemInlineSegments(segments []inlineSegment, x, y float32, widget *HTMLWidget, result *RenderResult) {
	currentX := x
	
	for _, segment := range segments {
		widget.renderTextWithUnicode(segment.text, currentX, y, segment.font, segment.color)
		
		segmentWidth := widget.measureTextWidth(segment.font, segment.text, float32(segment.font.BaseSize))
		
		// BUG #5 FIX: Create link areas in screen coordinates
		if segment.href != "" {
			fontSize := float32(segment.font.BaseSize)
			if fontSize == 0 {
				fontSize = 16
			}
			
			bounds := rl.NewRectangle(currentX, y, segmentWidth, fontSize)
			linkArea := LinkArea{Bounds: bounds, URL: segment.href}
			result.LinkAreas = append(result.LinkAreas, linkArea)
			
			// Draw underline for links
			rl.DrawLineEx(
				rl.NewVector2(currentX, y+fontSize),
				rl.NewVector2(currentX+segmentWidth, y+fontSize),
				1, segment.color)
		}
		
		currentX += segmentWidth
	}
}

// HRRenderHandler handles horizontal rules
type HRRenderHandler struct{}

func (h *HRRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "hr"
}

func (h *HRRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	y := ctx.Y + 10
	
	// BUG #6 FIX: Use proper margin calculations for HR width
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

// BreakRenderHandler handles br elements
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

// PreRenderHandler handles preformatted text blocks
type PreRenderHandler struct{}

func (h *PreRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "pre"
}

func (h *PreRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	// Collect all text content
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

	// BUG #6 FIX: Use proper margin calculations for pre-block width
	rightMargin := ctx.Widget.BodyMargin + ctx.Widget.BodyPadding
	blockWidth := ctx.Width - rightMargin

	// Draw background
	backgroundRect := rl.NewRectangle(ctx.X, y, blockWidth, blockHeight)
	rl.DrawRectangleRec(backgroundRect, rl.Color{R: 248, G: 248, B: 248, A: 255})
	rl.DrawRectangleLinesEx(backgroundRect, 1, rl.Color{R: 220, G: 220, B: 220, A: 255})

	// Render lines
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

// CodeRenderHandler handles code elements with context awareness
type CodeRenderHandler struct{}

func (h *CodeRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "code"
}

func (h *CodeRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
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

	// Context-aware rendering
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

	// BUG #6 FIX: Use proper margin calculations
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

	// BUG #4 FIX: Handle inline positioning
	renderX := ctx.CurrentX
	if renderX == 0 {
		renderX = ctx.X
	}

	// Draw background
	backgroundRect := rl.NewRectangle(renderX-padding, ctx.Y-2, textSize.X+2*padding, textSize.Y+4)
	rl.DrawRectangleRec(backgroundRect, rl.Color{R: 240, G: 240, B: 240, A: 255})
	rl.DrawRectangleLinesEx(backgroundRect, 1, rl.Color{R: 220, G: 220, B: 220, A: 255})

	ctx.Widget.renderTextWithUnicode(content, renderX, ctx.Y, font, rl.Color{R: 40, G: 40, B: 40, A: 255})

	if ctx.CurrentX > 0 {
		// Inline context - advance X
		return RenderResult{
			NextY:      ctx.Y,
			NextX:      renderX + textSize.X + 2*padding,
			Height:     textSize.Y + 5,
			LineHeight: textSize.Y,
		}
	} else {
		// Block context - advance Y
		return RenderResult{
			NextY:  ctx.Y + textSize.Y + 5,
			Height: textSize.Y + 5,
		}
	}
}

// GLOBAL FONT MANAGER (UNCHANGED)

type GlobalFontManager struct {
	fonts         map[string]rl.Font
	refCounts     map[string]int
	mutex         sync.RWMutex
	fontPaths     map[string]string
	monoFontPaths map[string]string
	initialized   bool
}

var fontManager *GlobalFontManager
var fontManagerOnce sync.Once

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

func (fm *GlobalFontManager) initializePlatformPaths() {
	if runtime.GOOS == "darwin" {
		fm.fontPaths["arial"] = "/System/Library/Fonts/Supplemental/Arial.ttf"
		fm.fontPaths["arial-bold"] = "/System/Library/Fonts/Supplemental/Arial Bold.ttf"
		fm.fontPaths["arial-italic"] = "/System/Library/Fonts/Supplemental/Arial Italic.ttf"
		fm.monoFontPaths["monaco"] = "/System/Library/Fonts/Monaco.ttf"
		fm.monoFontPaths["menlo"] = "/System/Library/Fonts/Menlo.ttc"
		fm.monoFontPaths["courier"] = "/System/Library/Fonts/Courier.ttc"
	} else if runtime.GOOS == "windows" {
		fm.fontPaths["arial"] = "C:/Windows/Fonts/arial.ttf"
		fm.fontPaths["arial-bold"] = "C:/Windows/Fonts/arialbd.ttf"
		fm.fontPaths["arial-italic"] = "C:/Windows/Fonts/ariali.ttf"
		fm.monoFontPaths["consolas"] = "C:/Windows/Fonts/consola.ttf"
		fm.monoFontPaths["cascadia"] = "C:/Windows/Fonts/CascadiaCode.ttf"
		fm.monoFontPaths["courier"] = "C:/Windows/Fonts/cour.ttf"
		fm.monoFontPaths["lucida-console"] = "C:/Windows/Fonts/lucon.ttf"
	} else {
		fm.fontPaths["arial"] = "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf"
		fm.fontPaths["arial-bold"] = "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf"
		fm.fontPaths["arial-italic"] = "/usr/share/fonts/truetype/liberation/LiberationSans-Italic.ttf"
		fm.monoFontPaths["dejavu-mono"] = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
		fm.monoFontPaths["liberation-mono"] = "/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf"
		fm.monoFontPaths["ubuntu-mono"] = "/usr/share/fonts/truetype/ubuntu/UbuntuMono-R.ttf"
		fm.monoFontPaths["courier"] = "/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf"
	}
	fm.initialized = true
}

func (fm *GlobalFontManager) GetMonospaceFont(size int32) rl.Font {
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

	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if font, exists := fm.fonts[key]; exists {
		fm.refCounts[key]++
		return font
	}

	var fontOrder []string
	if runtime.GOOS == "darwin" {
		fontOrder = []string{"monaco", "menlo", "sf-mono", "courier"}
	} else if runtime.GOOS == "windows" {
		fontOrder = []string{"consolas", "cascadia", "courier", "lucida-console"}
	} else {
		fontOrder = []string{"dejavu-mono", "liberation-mono", "ubuntu-mono", "courier"}
	}

	var loadedFont rl.Font

	for _, fontName := range fontOrder {
		if fontPath, exists := fm.monoFontPaths[fontName]; exists {
			testFont := rl.LoadFontEx(fontPath, size, essentialCodepoints)
			if testFont.BaseSize > 0 {
				loadedFont = testFont
				break
			}
		}
	}

	if loadedFont.BaseSize == 0 {
		loadedFont = rl.GetFontDefault()
	}

	fm.fonts[key] = loadedFont
	fm.refCounts[key] = 1

	return loadedFont
}

func (fm *GlobalFontManager) GetFont(fontName string, size int32) rl.Font {
	key := fmt.Sprintf("%s:%d", fontName, size)

	fm.mutex.RLock()
	if font, exists := fm.fonts[key]; exists {
		fm.mutex.RUnlock()
		fm.mutex.Lock()
		fm.refCounts[key]++
		fm.mutex.Unlock()
		return font
	}
	fm.mutex.RUnlock()

	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if font, exists := fm.fonts[key]; exists {
		fm.refCounts[key]++
		return font
	}

	fontPath, pathExists := fm.fontPaths[fontName]
	if !pathExists {
		defaultFont := rl.GetFontDefault()
		fm.fonts[key] = defaultFont
		fm.refCounts[key] = 1
		return defaultFont
	}

	font := rl.LoadFontEx(fontPath, size, essentialCodepoints)
	if font.BaseSize == 0 {
		font = rl.GetFontDefault()
	}

	fm.fonts[key] = font
	fm.refCounts[key] = 1

	return font
}

func (fm *GlobalFontManager) ReleaseFont(fontName string, size int32) {
	key := fmt.Sprintf("%s:%d", fontName, size)

	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if count, exists := fm.refCounts[key]; exists {
		count--
		if count <= 0 {
			if font, fontExists := fm.fonts[key]; fontExists {
				defaultFont := rl.GetFontDefault()
				if font.BaseSize > 0 && font.Texture.ID != defaultFont.Texture.ID {
					rl.UnloadFont(font)
				}
			}
			delete(fm.fonts, key)
			delete(fm.refCounts, key)
		} else {
			fm.refCounts[key] = count
		}
	}
}

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
				}
			}
			delete(fm.fonts, key)
			delete(fm.refCounts, key)
		} else {
			fm.refCounts[key] = count
		}
	}
}

// TEXT MEASUREMENT CACHE (UNCHANGED)

type TextMeasureCache struct {
	cache       map[string]rl.Vector2
	accessOrder []string
	maxEntries  int
}

func NewTextMeasureCache(maxEntries int) *TextMeasureCache {
	return &TextMeasureCache{
		cache:       make(map[string]rl.Vector2),
		accessOrder: make([]string, 0),
		maxEntries:  maxEntries,
	}
}

func (tmc *TextMeasureCache) GetTextSize(font rl.Font, text string, fontSize float32) rl.Vector2 {
	key := fmt.Sprintf("%d:%.1f:%s", font.Texture.ID, fontSize, text)

	if size, exists := tmc.cache[key]; exists {
		tmc.updateAccessOrder(key)
		return size
	}

	size := rl.MeasureTextEx(font, text, fontSize, 1)

	tmc.cache[key] = size
	tmc.accessOrder = append(tmc.accessOrder, key)

	if len(tmc.cache) > tmc.maxEntries {
		oldestKey := tmc.accessOrder[0]
		delete(tmc.cache, oldestKey)
		tmc.accessOrder = tmc.accessOrder[1:]
	}

	return size
}

func (tmc *TextMeasureCache) GetTextWidth(font rl.Font, text string, fontSize float32) float32 {
	return tmc.GetTextSize(font, text, fontSize).X
}

func (tmc *TextMeasureCache) updateAccessOrder(key string) {
	for i, k := range tmc.accessOrder {
		if k == key {
			tmc.accessOrder = append(tmc.accessOrder[:i], tmc.accessOrder[i+1:]...)
			break
		}
	}
	tmc.accessOrder = append(tmc.accessOrder, key)
}

func (tmc *TextMeasureCache) Clear() {
	tmc.cache = make(map[string]rl.Vector2)
	tmc.accessOrder = tmc.accessOrder[:0]
}

// LEGACY COMPATIBILITY STRUCTURES (for API preservation)

type HTMLElement struct {
	Tag      string
	Content  string
	Href     string
	Level    int
	Bold     bool
	Italic   bool
	Children []HTMLElement
}

type FontSet struct {
	Regular        rl.Font
	Bold           rl.Font
	Italic         rl.Font
	BoldItalic     rl.Font
	H1             rl.Font
	H2             rl.Font
	H3             rl.Font
	H4             rl.Font
	H5             rl.Font
	H6             rl.Font
	Monospace      rl.Font
	MonospaceLarge rl.Font
}

type LinkArea struct {
	Bounds rl.Rectangle
	URL    string
	Hover  bool
}

// HTMLWidget - API PRESERVED, internals modernized
type HTMLWidget struct {
	// PRESERVED API FIELDS
	Content        string
	Elements       []HTMLElement // Populated for API compatibility only
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

	// INTERNAL IMPLEMENTATION
	document  HTMLDocument
	parser    *StateMachineParser
	renderer  *HTMLRenderer
	textCache *TextMeasureCache
}

// NewHTMLWidget creates a new HTML widget - API UNCHANGED
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
}

func (w *HTMLWidget) measureText(font rl.Font, text string, fontSize float32) rl.Vector2 {
	return w.textCache.GetTextSize(font, text, fontSize)
}

func (w *HTMLWidget) measureTextWidth(font rl.Font, text string, fontSize float32) float32 {
	return w.textCache.GetTextWidth(font, text, fontSize)
}

func (w *HTMLWidget) parseHTML(html string) {
	w.document = w.parser.Parse(html)

	// Populate Elements field for API compatibility (but don't use for rendering)
	w.Elements = w.createLegacyElementsForAPI()
}

// createLegacyElementsForAPI creates legacy elements for API compatibility only
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

// BUG #9 FIX: Enhanced Unicode rendering with improved character width calculation
func (w *HTMLWidget) renderTextWithUnicode(text string, x, y float32, font rl.Font, color rl.Color) {
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16
	}

	hasUnicode := false
	for _, r := range text {
		if r >= 128 {
			hasUnicode = true
			break
		}
	}

	if !hasUnicode {
		rl.DrawTextEx(font, text, rl.NewVector2(x, y), fontSize, 1, color)
		return
	}

	currentX := x
	runes := []rune(text)

	for _, r := range runes {
		if r < 128 {
			charStr := string(r)
			charWidth := w.measureTextWidth(font, charStr, fontSize)
			rl.DrawTextEx(font, charStr, rl.NewVector2(currentX, y), fontSize, 1, color)
			currentX += charWidth
		} else {
			// BUG #9 FIX: Improved Unicode character width calculation
			charWidth := w.calculateUnicodeCharWidth(r, fontSize)
			rl.DrawTextCodepoint(font, r, rl.NewVector2(currentX, y), fontSize, color)
			currentX += charWidth
		}
	}
}

// BUG #9 FIX: Better Unicode character width calculation
func (w *HTMLWidget) calculateUnicodeCharWidth(r rune, fontSize float32) float32 {
	// More accurate width calculation based on character categories
	switch {
	// Latin accented characters (À-ÿ) - similar width to ASCII
	case r >= 0x00C0 && r <= 0x00FF:
		return fontSize * 0.55
	
	// Latin Extended-A (Ā-ſ) - slightly wider
	case r >= 0x0100 && r <= 0x017F:
		return fontSize * 0.58
	
	// Common punctuation - narrower
	case r == 0x2013 || r == 0x2014: // dashes
		return fontSize * 0.5
	case r == 0x2018 || r == 0x2019 || r == 0x201C || r == 0x201D: // quotes
		return fontSize * 0.3
	case r == 0x2026: // ellipsis
		return fontSize * 0.8
	case r == 0x00AB || r == 0x00BB: // French angle quotes
		return fontSize * 0.45
	
	// Bullet points and symbols
	case r == 0x2022 || r == 0x25CF:
		return fontSize * 0.4
	
	// Default fallback for unknown Unicode characters
	default:
		return fontSize * 0.6
	}
}

func (w *HTMLWidget) renderText(text string, x, y, width float32, font rl.Font, color rl.Color) float32 {
	if text == "" {
		return y
	}

	words := strings.Fields(text)
	currentLine := ""
	lineHeight := float32(20)
	currentY := y
	
	// BUG #6 FIX: Use proper margin calculations for text wrapping
	rightMargin := w.BodyMargin + w.BodyPadding

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		textWidth := w.measureTextWidth(font, testLine, float32(font.BaseSize))
		// BUG #6 FIX: Use calculated margin instead of hardcoded 40
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

// BUG #5 FIX: Enhanced Update method with correct link coordinate handling
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

		// BUG #5 FIX: Adjust link bounds for scroll position during hit testing
		screenBounds := area.Bounds
		screenBounds.Y -= w.ScrollY // Subtract scroll to convert to screen coordinates

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

// BUG #7 & #8 FIX: Enhanced Render method with proper height and scrollbar positioning
func (w *HTMLWidget) Render(x, y, width, height float32) {
	w.LinkAreas = w.LinkAreas[:0]
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

	// BUG #2 FIX: Ensure proper font context initialization
	ctx := RenderContext{
		X:           contentX,
		Y:           contentY,
		Width:       contentWidth,
		ParentFont:  w.Fonts.Regular, // Ensure this is a valid loaded font
		ParentColor: rl.Black,
		Widget:      w,
		CurrentX:    contentX, // BUG #4 FIX: Initialize X position tracking
	}

	result := w.renderer.RenderDocument(w.document, ctx)

	// BUG #7 FIX: Always recalculate total height instead of only once
	w.TotalHeight = result.NextY + w.ScrollY - contentY + 2*(w.BodyMargin+w.BodyPadding)

	// BUG #5 FIX: Link areas are now in correct screen coordinates, just adjust for scroll offset
	for _, linkArea := range result.LinkAreas {
		// Convert to screen coordinates by adding scroll offset
		screenArea := linkArea
		screenArea.Bounds.Y += w.ScrollY
		w.LinkAreas = append(w.LinkAreas, screenArea)
	}

	rl.EndScissorMode()

	if w.TotalHeight > height {
		w.drawScrollbar(x, y, width, height)
	}
}

// BUG #8 FIX: Enhanced scrollbar with proper margin-aware positioning
func (w *HTMLWidget) drawScrollbar(x, y, width, height float32) {
	if w.TotalHeight <= height || w.ScrollbarAlpha <= 0.01 {
		return
	}

	scrollbarWidth := float32(10)
	
	// BUG #8 FIX: Position scrollbar accounting for content margins
	contentMargin := w.BodyMargin + w.BodyPadding
	scrollbarX := x + width - scrollbarWidth - contentMargin

	// BUG #8 FIX: Scrollbar track should match content area
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

// Unload - API UNCHANGED
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

// EXTENSION API - ENHANCED

// RegisterRenderHandler allows adding custom render handlers
func (w *HTMLWidget) RegisterRenderHandler(elementType string, handler RenderHandler) {
	w.renderer.RegisterHandler(elementType, handler)
}

// GetRenderer returns the widget's renderer for advanced usage
func (w *HTMLWidget) GetRenderer() *HTMLRenderer {
	return w.renderer
}

// GetDocument returns the parsed document for advanced usage
func (w *HTMLWidget) GetDocument() HTMLDocument {
	return w.document
}

// DebugDocument prints the document structure once for debugging (call manually)
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