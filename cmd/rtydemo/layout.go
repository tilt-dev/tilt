package main

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/hud/view"
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
