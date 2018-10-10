package rty

import (
	"math"

	"github.com/windmilleng/tcell"
)

// Components are able to render themselves onto a screen

type ID string

type FQID string

type RTY interface {
	Render(c Component) error
	TextScroller(FQID) TextScroller
}

type TextScroller interface {
	Up()
	Down()
	PgUp()
	PgDn()
	Home()
	End()
}

// Component renders onto a canvas
type Component interface {
	ID() ID
	Size(availWidth, availHeight int) (int, int)
	Render(w Writer, width, height int) error
}

type Writer interface {
	SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style)

	Divide(x, y, width, height int) Writer

	RenderChild(c Component) int

	RenderChildInTemp(c Component) (Canvas, *LineProvenanceData)
	Embed(src Canvas, srcY, srcHeight int)

	RenderChildScroll(c ScrollComponent)
}

type LineProvenanceData struct {
	Data []FQID
}

const GROW = math.MaxInt32
