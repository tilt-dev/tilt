package main

import (
	"fmt"

	"github.com/windmilleng/tcell"

	"github.com/windmilleng/tilt/internal/rty"
)

func (d *Demo) Layout() rty.Component {
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
	b := rty.NewBox()
	b.SetInner(rty.TextString("header"))
	c := rty.Bg(rty.Fg(b, tcell.ColorBlack), tcell.ColorOrange)
	return rty.NewFixedSize(c, rty.GROW, 7)
}

func (d *Demo) resources() rty.Component {
	childNames := make([]string, len(d.view.Resources))
	for i, r := range d.view.Resources {
		childNames[i] = r.Name
	}
	l, selectedResource := d.rty.RegisterElementScroll("resources", childNames)

	for i, r := range d.view.Resources {
		l.Add(d.resource(r, selectedResource == r.Name, i%2 == 0))
	}

	b := rty.NewBox()
	if d.nav.selectedPane == selectedResources {
		b.SetFocused(true)
	}
	b.SetInner(l)
	return b
}

func (d *Demo) resource(r Resource, selected bool, even bool) rty.Component {
	lines := rty.NewLines()
	cl := rty.NewLine()
	cl.Add(rty.TextString(r.Name))
	// cl.Add(rty.NewFillerString('-'))
	cl.Add(rty.TextString(fmt.Sprintf("%d", r.Status)))
	lines.Add(cl)
	cl = rty.NewLine()
	cl.Add(rty.TextString(fmt.Sprintf(
		"LOCAL: (watching %v) - ", r.DirectoryWatched)))
	// cl.Add(rty.NewTruncatingStrings(r.LatestFileChanges))
	lines.Add(cl)
	cl = rty.NewLine()
	cl.Add(rty.TextString(fmt.Sprintf("  K8S: %v", r.StatusDesc)))
	lines.Add(cl)
	cl = rty.NewLine()
	lines.Add(cl)
	cl = rty.NewLine()
	cl.Add(rty.TextString("padding"))
	lines.Add(cl)
	cl = rty.NewLine()
	cl.Add(rty.TextString("padding2"))
	lines.Add(cl)

	b := rty.NewBox()
	b.SetInner(lines)

	var c rty.Component
	c = b
	if even {
		c = rty.Fg(rty.Bg(c, tcell.ColorWhite), tcell.ColorBlack)
	}
	return c
}

func (d *Demo) stream() rty.Component {
	c := rty.NewScrollingWrappingTextArea("stream", longText)

	b := rty.NewBox()
	b.SetInner(c)
	if d.nav.selectedPane == selectedStream {
		b.SetFocused(true)
	}
	return b
}

func (d *Demo) footer() rty.Component {
	b := rty.NewBox()
	b.SetInner(rty.TextString("footer"))

	c := rty.Bg(rty.Fg(b, tcell.ColorOrange), tcell.ColorBlack)
	return rty.NewFixedSize(c, rty.GROW, 7)
}
