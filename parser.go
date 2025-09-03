package marquee

import (
	"strings"
)

type HTMLNode struct {
	Type       NodeType
	Tag        string
	Content    string
	Attributes map[string]string
	Children   []HTMLNode
	Context    NodeContext
	Parent     *HTMLNode
}

type HTMLDocument struct {
	Root     HTMLNode
	Metadata DocumentMetadata
}

type DocumentMetadata struct {
	Title       string
	Scripts     []ScriptInfo
	StyleSheets []StyleInfo
	MetaTags    []MetaInfo
	DocType     string
}

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

	maxDepth     int
	currentDepth int
	maxLength    int

	errorCount int
	maxErrors  int
}

type NodeStackEntry struct {
	Node        *HTMLNode
	OriginalTag string
}

func NewStateMachineParser() *StateMachineParser {
	return &StateMachineParser{
		currentAttrs: make(map[string]string),
		maxDepth:     50,
		maxLength:    1000000,
		maxErrors:    100,
	}
}

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
	p.errorCount = 0
}

func (p *StateMachineParser) handleParseError(message string) bool {
	p.errorCount++
	if p.errorCount > p.maxErrors {

		return false
	}

	p.state = StateText
	p.tagBuffer.Reset()
	p.attrValue.Reset()
	p.currentAttrs = make(map[string]string)

	return true
}

func (p *StateMachineParser) Parse(html string) HTMLDocument {

	p.Reset()

	if len(html) == 0 {
		return HTMLDocument{Root: HTMLNode{Type: NodeTypeDocument, Context: ContextRoot}}
	}

	if len(html) > p.maxLength {
		html = html[:p.maxLength]
	}

	p.input = []rune(strings.TrimSpace(html))
	p.position = 0
	p.state = StateText

	root := &HTMLNode{
		Type:       NodeTypeDocument,
		Context:    ContextRoot,
		Attributes: make(map[string]string),
		Children:   make([]HTMLNode, 0),
	}
	p.nodeStack = []NodeStackEntry{{Node: root, OriginalTag: "document"}}

	for p.position < len(p.input) {
		char := p.input[p.position]

		if p.currentDepth > p.maxDepth {
			if !p.handleParseError("max depth exceeded") {
				break
			}
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

		if p.position > len(p.input) {
			break
		}
	}

	if p.textBuffer.Len() > 0 {
		p.addTextNode(p.textBuffer.String())
	}

	for len(p.nodeStack) > 1 {
		p.nodeStack = p.nodeStack[:len(p.nodeStack)-1]
	}

	return HTMLDocument{Root: *root}
}

func (p *StateMachineParser) handleTextState(char rune) {
	if char == '<' {

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

	} else {

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

func (p *StateMachineParser) handleCommentState(char rune) {

	if char == '>' && p.position >= 2 {
		if p.position-1 < len(p.input) && p.position-2 < len(p.input) &&
			p.input[p.position-1] == '-' && p.input[p.position-2] == '-' {
			p.state = StateText
		}
	}

	if p.position > 1000 {
		p.state = StateText
	}
}

func (p *StateMachineParser) addTextNode(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}

	if len(p.nodeStack) == 0 {
		return
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

	if tagName == "" || len(tagName) > 20 {
		if !p.handleParseError("invalid tag name") {
			return
		}
		p.tagBuffer.Reset()
		p.currentAttrs = make(map[string]string)
		return
	}

	node := HTMLNode{
		Type:       NodeTypeElement,
		Tag:        tagName,
		Attributes: make(map[string]string),
		Children:   make([]HTMLNode, 0),
	}

	for k, v := range p.currentAttrs {
		node.Attributes[k] = v
	}

	if len(p.nodeStack) == 0 {
		return
	}

	parent := p.nodeStack[len(p.nodeStack)-1].Node
	node.Context = p.determineContext(tagName, parent)
	node.Parent = parent

	originalTag := tagName
	node = *p.normalizeElement(&node)

	parent.Children = append(parent.Children, node)

	if p.isContainerElement(originalTag) {

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

	if tagName == "" || len(p.nodeStack) == 0 || len(tagName) > 20 {
		if !p.handleParseError("invalid self-closing tag") {
			return
		}
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

	if tagName == "" || len(p.nodeStack) <= 1 || len(tagName) > 20 {
		p.tagBuffer.Reset()
		return
	}

	for i := len(p.nodeStack) - 1; i > 0; i-- {
		current := p.nodeStack[i]

		if current.Node.Tag == tagName || current.OriginalTag == tagName {

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
