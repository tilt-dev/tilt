package rty

import (
	"math"

	"github.com/windmilleng/tcell"
)

// Components are able to render themselves onto a screen

type RTY interface {
	Render(c Component) error

	// Register must be called before Render
	RegisterElementScroll(name string, children []string) (l *ElementScrollLayout, selectedChild string)

	// *Scroller must be called after Render (each call to Render invalidates previous Crollers)
	ElementScroller(name string) ElementScroller
	TextScroller(name string) TextScroller
}

type ElementScroller interface {
	Up()
	Down()
	Top()
	Bottom()
	GetSelectedIndex() int
}

type TextScroller interface {
	Up()
	Down()
	Top()
	Bottom()

	ToggleFollow()
	SetFollow(following bool)
}

// Component renders onto a canvas
type Component interface {
	Size(availWidth, availHeight int) (int, int)
	Render(w Writer, width, height int) error
}

type Writer interface {
	SetContent(x int, y int, mainc rune, combc []rune)

	Divide(x, y, width, height int) Writer
	Foreground(c tcell.Color) Writer
	Background(c tcell.Color) Writer
	Fill() Writer

	RenderChild(c Component) int

	RenderChildInTemp(c Component) Canvas
	Embed(src Canvas, srcY, srcHeight int)

	RenderStateful(c StatefulComponent, name string)
}

const GROW = math.MaxInt32
