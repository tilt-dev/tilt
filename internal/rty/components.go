package rty

import (
	"math"

	"github.com/windmilleng/tcell"
)

// Components are able to render themselves onto a screen

type RTY interface {
	Render(c Component) error
	// ElementScroller(FQID) ElementScroller
	TextScroller(name string) TextScroller
}

// type ElementScroller interface {
// 	GetSelection() string
// 	Select(string)
// 	Next()
// 	Prev()
// }

type TextScroller interface {
	Up()
	Down()
	// PgUp()
	// PgDn()
	// Home()
	// End()
}

// Component renders onto a canvas
type Component interface {
	Size(availWidth, availHeight int) (int, int)
	Render(w Writer, width, height int) error
}

type Writer interface {
	SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style)

	Divide(x, y, width, height int) Writer

	RenderChild(c Component) int

	RenderChildInTemp(c Component) Canvas
	Embed(src Canvas, srcY, srcHeight int)

	RenderStateful(c StatefulComponent, name string)
}

const GROW = math.MaxInt32
