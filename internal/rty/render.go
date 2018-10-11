package rty

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/windmilleng/tcell"
)

func NewRTY(screen tcell.Screen) RTY {
	return &rty{
		screen: screen,
	}
}

type rty struct {
	screen tcell.Screen
	state  renderState
}

type renderState map[FQID]interface{}

func (r *rty) Render(c Component) error {
	log.Printf("Render outer loop")
	r.screen.Clear()
	g := &renderGlobals{
		prev: r.state,
	}
	f := renderFrame{
		fqid:    "/",
		canvas:  newScreenCanvas(r.screen),
		globals: g,
	}

	log.Printf("showing")

	f.RenderChild(c)
	r.screen.Show()
	r.state = g.next
	return g.err
}

func (r *rty) TextScroller(FQID) TextScroller {
	return nil
}

type renderGlobals struct {
	err  error
	prev renderState
	next renderState
}

func (g *renderGlobals) Get(fqid FQID) interface{} {
	return g.prev[fqid]
}

func (g *renderGlobals) Set(fqid FQID, d interface{}) {
	g.next[fqid] = d
}

func (g *renderGlobals) errorf(format string, a ...interface{}) {
	if g.err != nil {
		return
	}
	g.err = fmt.Errorf(format, a...)
}

type renderFrame struct {
	fqid   FQID
	canvas Canvas
	lpw    *lineProvenanceWriter

	globals *renderGlobals
}

func (f renderFrame) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if err := f.canvas.SetContent(x, y, mainc, combc, style); err != nil {
		f.error(err)
	}
}

func (f renderFrame) Divide(x, y, width, height int) Writer {
	f.canvas = newSubCanvas(f.canvas, x, y, width, height)
	f.lpw = f.lpw.Divide(y)
	return f
}

func (f renderFrame) RenderChild(c Component) int {
	f = f.join(c.ID())

	width, height := f.canvas.Size()
	if err := c.Render(f, width, height); err != nil {
		f.error(err)
	}

	_, height = f.canvas.Close()
	f.lpw.WriteLineProvenance(f.fqid, 0, height)
	return height
}

func (f renderFrame) RenderChildInTemp(c Component) (Canvas, *LineProvenanceData) {
	f = f.join(c.ID())
	width, _ := f.canvas.Size()
	tmp := newTempCanvas(width, GROW)
	f.canvas = tmp

	lpw := newLineProvenanceWriter()
	f.lpw = lpw

	if err := c.Render(f, width, GROW); err != nil {
		f.error(err)
	}
	return tmp, lpw.data()
}

func (f renderFrame) Embed(src Canvas, srcY int, srcHeight int) {
	width, destLines := f.canvas.Size()
	srcLines := srcHeight - srcY

	numLines := destLines
	if srcLines < destLines {
		numLines = srcLines
	}
	for i := 0; i < numLines; i++ {
		for j := 0; j < width; j++ {
			mainc, combc, style, _ := src.GetContent(j, srcY+i)
			f.SetContent(j, i, mainc, combc, style)
		}
	}
}

func (f renderFrame) RenderChildScroll(c ScrollComponent) {
	f = f.join(c.ID())
	prev := f.globals.Get(f.fqid)

	width, height := f.canvas.Size()
	next, err := c.Render(f, prev, width, height)
	if err != nil {
		f.error(err)
	}

	f.globals.Set(f.fqid, next)
}

func (f renderFrame) errorf(fmt string, a ...interface{}) {
	f.globals.errorf("%s"+fmt, append([]interface{}{f.fqid}, a...))
}

func (f renderFrame) error(err error) {
	f.globals.errorf("%s: %s", f.fqid, err.Error())
}

func (f renderFrame) join(id ID) renderFrame {
	if id != "" {
		f.fqid = FQID(filepath.Join(string(f.fqid), string(id)))
	}
	return f
}
