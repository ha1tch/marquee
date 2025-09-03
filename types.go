package marquee

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

var essentialCodepoints []rune

func init() {

	for i := rune(32); i <= 126; i++ {
		essentialCodepoints = append(essentialCodepoints, i)
	}

	for i := rune(0x00C0); i <= 0x00FF; i++ {
		essentialCodepoints = append(essentialCodepoints, i)
	}

	for i := rune(0x0100); i <= 0x017F; i++ {
		essentialCodepoints = append(essentialCodepoints, i)
	}

	essentialUnicode := []rune{
		0x2022,
		0x25CF,
		0x2013,
		0x2014,
		0x201C,
		0x201D,
		0x2018,
		0x2019,
		0x2026,
		0x00A0,
		0x00AB,
		0x00BB,
	}
	essentialCodepoints = append(essentialCodepoints, essentialUnicode...)
}

type NodeType int

const (
	NodeTypeText NodeType = iota
	NodeTypeElement
	NodeTypeDocument
)

type NodeContext int

const (
	ContextBlock NodeContext = iota
	ContextInline
	ContextRoot
)

type ResourceType int

const (
	ResourceTypePage ResourceType = iota
	ResourceTypeCSS
	ResourceTypeImage
	ResourceTypeFont
)

type inlineSegment struct {
	text  string
	font  rl.Font
	color rl.Color
	href  string
}

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
