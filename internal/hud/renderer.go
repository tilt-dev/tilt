package hud

import (
	"fmt"
	"os"

	"github.com/windmilleng/tcell"
	"github.com/windmilleng/tilt/internal/hud/view"
)

type Renderer struct {
	screen tcell.Screen
}

func (r *Renderer) Render(v view.View) error {
	if r.screen != nil {
		r.screen.Clear()
		p := newPen(r.screen)
		for _, res := range v.Resources {
			p.putln(fmt.Sprintf("%v", res))
		}
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

	r.screen = screen

	return nil
}

func (r *Renderer) Reset() {
	r.screen.Fini()
	r.screen = nil
}
