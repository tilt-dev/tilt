package rty

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

type renderState struct {
	compScrolls map[FQID]*scrollState
	textScrolls map[FQID]*textScrollState
}

type Writer interface {
	SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style)
	RenderChild(c Component)
	Embed(r Reader)

	Divide(x, y, width, height int) WriteCloser
	RecordLineProvenance() (Writer, LineProvenancer)
	Temp(width, height int) (WriteCloser, Reader)

	ReadPrevState() interface{}
	WriteState(interface{})
}

type Closer interface {
	Close() (int, int)
}

type LineProvenancer interface {
	LineProvenance() map[int]FQID
}

type Reader interface {
	Size() (int, int)
	GetContent(x, y int) (mainc rune, combc []rune, style tcell.Style, width int)
	Window(x, y, width, height) Reader
}

type RenderContext struct {
	prev *RenderState
	next *RenderState

	fqid FQID
}

func (rctx *RenderContext) Render(c Component) {
}

func (rctx *RenderContext) Sub(x, y, width, height int) *RenderContext {
}

func (rctx *RenderContext) Temp(c Component, width, height int) Rendered {
}

func (rctx *RenderContext) Embed(r Rendered, offset int) {
}

type compScrollState struct {
	width  int
	height int
	lines  map[int]FQID

	cursorID  FQID // which component is selected
	cursorIdx int  // which line within the selected component is selected
	posInPort int
}

type textScrollState struct {
	width  int
	height int
	lines  map[int]FQID

	tail bool

	cursorID  FQID
	cursorIdx int
}
