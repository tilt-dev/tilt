package rty

import (
	"fmt"

	"github.com/gdamore/tcell"
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

func (r *rty) Render(c Component) (err error) {
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

func (r *rty) RegisterElementScroll(name string, children []string) (l *ElementScrollLayout, selectedChild string) {
	r.state[name], selectedChild = adjustElementScroll(r.state[name], children)
	return &ElementScrollLayout{
		name: name,
	}, selectedChild
}

func (r *rty) ElementScroller(name string) ElementScroller {
	st, ok := r.state[name]
	if !ok {
		st = &ElementScrollState{}
		r.state[name] = st
	}

	return &ElementScrollController{state: st.(*ElementScrollState)}
}

func (r *rty) TextScroller(name string) TextScroller {
	st, ok := r.state[name]
	if !ok {
		st = &TextScrollState{}
		r.state[name] = st
	}

	return &TextScrollController{state: st.(*TextScrollState)}
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

var _ Writer = renderFrame{}

func (f renderFrame) SetContent(x int, y int, mainc rune, combc []rune) {
	if mainc == 0 {
		mainc = ' '
	}
	if err := f.canvas.SetContent(x, y, mainc, combc, f.style); err != nil {
		f.error(err)
	}
}

func (f renderFrame) Fill() (Writer, error) {
	width, height := f.canvas.Size()
	var err error
	f.canvas, err = newSubCanvas(f.canvas, 0, 0, width, height, f.style)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (f renderFrame) Divide(x, y, width, height int) (Writer, error) {
	var err error
	f.canvas, err = newSubCanvas(f.canvas, x, y, width, height, f.style)
	if err != nil {
		return nil, err
	}
	return f, nil
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

func (f renderFrame) Style(style tcell.Style) (Writer, error) {
	width, height := f.canvas.Size()
	var err error
	f.canvas, err = newSubCanvas(f.canvas, 0, 0, width, height, style)
	if err != nil {
		return nil, err
	}
	return f, nil
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

func (f renderFrame) Embed(src Canvas, srcY int, srcHeight int) error {
	width, destLines := f.canvas.Size()

	numLines := destLines
	if srcHeight < destLines {
		numLines = srcHeight
	}

	for i := 0; i < numLines; i++ {
		for j := 0; j < width; j++ {
			mainc, combc, style, _, err := src.GetContent(j, srcY+i)
			if err != nil {
				return err
			}
			if err := f.canvas.SetContent(j, i, mainc, combc, style); err != nil {
				f.error(err)
			}
		}
	}

	return nil
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
