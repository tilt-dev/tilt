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
	log.Printf("ahhh")

	d, err := NewDemo()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("starting\n")

	err = d.Run()
	if err != nil {
		log.Fatal(err)
	}
}

type Demo struct {
	view view.View
}

func NewDemo() (*Demo, error) {
	r := &Demo{}

	r.view = view.View{
		Resources: []view.Resource{
			view.Resource{
				"fe",
				"fe",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
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
	for {
		screen.Clear()
		c := d.TopLevel()
		c.Render(rty.NewScreenCanvas(screen))
		screen.Show()
	}

	return nil
}

func (d *Demo) TopLevel() rty.Component {
	l := rty.NewFlexLayout(rty.DirVert)

	l.Add(d.header())
	l.Add(d.resources())
	l.Add(d.footer())

	return l
}

func (d *Demo) header() rty.FixedDimComponent {
	b := rty.NewBox()
	b.SetInner(rty.String("header"))
	return rty.NewFixedDimSize(b, 3)
}

func (d *Demo) resources() rty.Component {
	l := rty.NewScrollLayout(rty.DirVert)

	for _, r := range d.view.Resources {
		rc := d.resource(r)
		l.Add(rc)
	}

	return l
}

func (d *Demo) resource(r view.Resource) rty.FixedDimComponent {
	lines := rty.NewLines()
	cl := rty.NewLine()
	cl.Add(rty.String(r.Name))
	cl.Add(rty.NewFillerString('-'))
	cl.Add(rty.String(fmt.Sprintf("%d", r.Status)))
	lines.Add(cl)
	cl = rty.NewLine()
	cl.Add(rty.String(fmt.Sprintf(
		"LOCAL: (watching %v) - ", r.DirectoryWatched)))
	cl.Add(rty.NewTruncatingStrings(r.LatestFileChanges))
	lines.Add(cl)
	cl = rty.NewLine()
	cl.Add(rty.String(
		fmt.Sprintf("  K8S: %v", r.StatusDesc)))
	lines.Add(cl)
	cl = rty.NewLine()
	return lines
}

func (d *Demo) footer() rty.FixedDimComponent {
	b := rty.NewBox()
	b.SetInner(rty.String("footer"))

	return rty.NewFixedDimSize(b, 3)
}
