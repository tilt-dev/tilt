package main

import (
	"log"
	"os"

	"github.com/windmilleng/tcell"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
)

func main() {
	f, err := os.Create("logfile")
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(f)

	d, err := NewDemo()
	if err != nil {
		log.Fatal(err)
	}

	err = d.Run()
	if err != nil {
		log.Fatal(err)
	}
}

type Demo struct {
	view view.View
	rty  rty.RTY
	nav  *navState
}

func NewDemo() (*Demo, error) {
	r := &Demo{
		view: sampleView(),
		nav: &navState{
			selectedPane: selectedResources,
		},
	}

	return r, nil
}

func (d *Demo) Run() error {
	screen, err := tcell.NewTerminfoScreen()
	if err != nil {
		return err
	}
	screen.Init()
	defer screen.Fini()
	d.rty = rty.NewRTY(screen)
	screenEvs := make(chan tcell.Event)
	go func() {
		for {
			screenEvs <- screen.PollEvent()
		}
	}()

	// initial render
	if err := d.render(); err != nil {
		return err
	}

	for {
		select {
		case ev := <-screenEvs:
			done := d.handleScreenEvent(ev)
			if done {
				return nil
			}
		}
		if err := d.render(); err != nil {
			return err
		}
	}

	return nil
}

func (d *Demo) render() error {
	return d.rty.Render(d.Layout())
}
