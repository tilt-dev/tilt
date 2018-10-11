package main

import (
	"fmt"
	"log"
	"os"
	"time"

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
	return d.rty.Render(d.TopLevel())
}

func (d *Demo) TopLevel() rty.Component {
	l := rty.NewFlexLayout(rty.DirVert)

	l.Add(d.header())
	split := rty.NewFlexLayout(rty.DirHor)

	split.Add(d.resources())
	split.Add(d.stream())
	l.Add(split)
	l.Add(d.footer())

	return l
}

func (d *Demo) header() rty.Component {
	b := rty.NewBox("")
	b.SetInner(rty.String("", "header"))
	return rty.NewFixedSize("", b, rty.GROW, 7)
}

func (d *Demo) resources() rty.Component {
	l := rty.NewTextScrollLayout("resources")

	for _, r := range d.view.Resources {
		rc := d.resource(r)
		l.Add(rc)
	}

	b := rty.NewBox("")
	if d.nav.selectedPane == selectedResources {
		b.SetFocused(true)
	}
	b.SetInner(rty.NewWrapScrollComponent(l))
	return b
}

func (d *Demo) resource(r view.Resource) rty.Component {
	lines := rty.NewLines("")
	cl := rty.NewLine("")
	cl.Add(rty.String("", r.Name))
	cl.Add(rty.NewFillerString("", '-'))
	cl.Add(rty.String("", fmt.Sprintf("%d", r.Status)))
	lines.Add(cl)
	cl = rty.NewLine("")
	cl.Add(rty.String("", fmt.Sprintf(
		"LOCAL: (watching %v) - ", r.DirectoryWatched)))
	cl.Add(rty.NewTruncatingStrings("", r.LatestFileChanges))
	lines.Add(cl)
	cl = rty.NewLine("")
	cl.Add(rty.String("",
		fmt.Sprintf("  K8S: %v", r.StatusDesc)))
	lines.Add(cl)
	cl = rty.NewLine("")
	lines.Add(cl)
	cl = rty.NewLine("")
	cl.Add(rty.String("", "padding"))
	lines.Add(cl)
	cl = rty.NewLine("")
	cl.Add(rty.String("", "padding2"))
	lines.Add(cl)

	b := rty.NewBox("")
	b.SetInner(lines)
	return b
}

func (d *Demo) stream() rty.Component {
	b := rty.NewBox("")
	if d.nav.selectedPane == selectedStream {
		b.SetFocused(true)
	}
	b.SetInner(rty.NewScrollingWrappingTextArea("stream", longText))
	return b
}

func (d *Demo) footer() rty.Component {
	b := rty.NewBox("")
	b.SetInner(rty.String("", "footer"))

	return rty.NewFixedSize("", b, rty.GROW, 7)
}
