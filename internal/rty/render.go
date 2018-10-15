package rty

import (
	"fmt"

	"github.com/windmilleng/tcell"
)

func NewRTY(screen tcell.Screen) RTY {
	return &rty{
		screen: screen,
		state:  make(renderState),
	}
}

type rty struct {
	screen tcell.Screen
	state  renderState
}

type renderState map[string]interface{}

func (r *rty) Render(c Component) error {
	r.screen.Clear()
	g := &renderGlobals{
		prev: r.state,
		next: make(renderState),
	}
	f := renderFrame{
		canvas:  newScreenCanvas(r.screen),
		globals: g,
	}

	f.RenderChild(c)
	r.screen.Show()
	r.state = g.next
	return g.err
}

func (r *rty) TextScroller(key string) TextScroller {
	st, ok := r.state[key]
	if !ok {
		return nil
	}
	return NewTextScrollController(st.(*TextScrollState))
}

type renderGlobals struct {
	err  error
	prev renderState
	next renderState
}

func (g *renderGlobals) Get(key string) interface{} {
	return g.prev[key]
}

func (g *renderGlobals) Set(key string, d interface{}) {
	g.next[key] = d
}

func (g *renderGlobals) errorf(format string, a ...interface{}) {
	if g.err != nil {
		return
	}
	g.err = fmt.Errorf(format, a...)
}

type renderFrame struct {
	canvas Canvas

	style tcell.Style

	globals *renderGlobals
}

func (f renderFrame) SetContent(x int, y int, mainc rune, combc []rune) {
	if err := f.canvas.SetContent(x, y, mainc, combc, f.style); err != nil {
		f.error(err)
	}
}

func (f renderFrame) Fill() Writer {
	width, height := f.canvas.Size()
	f.canvas = newSubCanvas(f.canvas, 0, 0, width, height, f.style)
	return f
}

func (f renderFrame) Divide(x, y, width, height int) Writer {
	f.canvas = newSubCanvas(f.canvas, x, y, width, height, f.style)
	return f
}

func (f renderFrame) Foreground(c tcell.Color) Writer {
	f.style = f.style.Foreground(c)
	return f
}

func (f renderFrame) Background(c tcell.Color) Writer {
	f.style = f.style.Background(c)
	return f
}

func (f renderFrame) RenderChild(c Component) int {
	width, height := f.canvas.Size()
	if err := c.Render(f, width, height); err != nil {
		f.error(err)
	}

	_, height = f.canvas.Close()
	return height
}

func (f renderFrame) Style(style tcell.Style) Writer {
	width, height := f.canvas.Size()
	f.canvas = newSubCanvas(f.canvas, 0, 0, width, height, style)
	return f
}

func (f renderFrame) RenderChildInTemp(c Component) Canvas {
	width, _ := f.canvas.Size()
	tmp := newTempCanvas(width, GROW, f.style)
	f.canvas = tmp

	if err := c.Render(f, width, GROW); err != nil {
		f.error(err)
	}
	tmp.Close()
	return tmp
}

func (f renderFrame) Embed(src Canvas, srcY int, srcHeight int) {
	width, destLines := f.canvas.Size()

	numLines := destLines
	if srcHeight < destLines {
		numLines = srcHeight
	}

	for i := 0; i < numLines; i++ {
		for j := 0; j < width; j++ {
			mainc, combc, style, _ := src.GetContent(j, srcY+i)
			f.canvas.SetContent(j, i, mainc, combc, style)
		}
	}
}

func (f renderFrame) RenderStateful(c StatefulComponent, name string) {
	prev := f.globals.Get(name)

	width, height := f.canvas.Size()
	next, err := c.RenderStateful(f, prev, width, height)
	if err != nil {
		f.error(err)
	}

	f.globals.Set(name, next)
}

func (f renderFrame) errorf(fmt string, a ...interface{}) {
	f.globals.errorf(fmt, a...)
}

func (f renderFrame) error(err error) {
	f.globals.errorf("%s", err.Error())
}
