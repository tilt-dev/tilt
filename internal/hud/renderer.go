package hud

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/windmilleng/tcell"
	"github.com/windmilleng/tilt/internal/hud/view"
)

type Renderer struct {
	screen tcell.Screen
	ctx    context.Context // idk the best way to use this context, or even if it should be here
}

func (r *Renderer) Render(v view.View) error {
	if r.screen != nil {
		p := newPen(r.screen)
		p.putln(fmt.Sprintf("Rendered at: %s", time.Now().String()))
		r.screen.Show()
	}
	return nil
}

func (r *Renderer) SetUp(event ReadyEvent) error {
	// TODO(maia): support sigwinch
	// TODO(maia): pass term name along with ttyPath via RPC. Temporary hack:
	// get termName from current terminal, assume it's the same 🙈
	screen, err := tcell.NewScreenFromTty(event.ttyPath, nil, os.Getenv("TERM"))
	if err != nil {
		return err
	}
	if err = screen.Init(); err != nil {
		return err
	}
	r.screen = screen

	// janky code to exit ever (stolen from tcell demos)
	go func() {
		for {
			ev := screen.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyEscape, tcell.KeyEnter:
					// TODO: tell `tilt hud` to exit
					screen.Fini()
				}
			}
		}
	}()

	r.ctx = nil
	return nil
}

func (r *Renderer) Reset() {
	r.screen.Fini()
	r.screen = nil
	r.ctx = nil
}
