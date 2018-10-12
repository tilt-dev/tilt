package main

import (
	"github.com/windmilleng/tcell"

	"github.com/windmilleng/tilt/internal/rty"
)

type paneSelection int

const (
	selectedResources paneSelection = iota
	selectedStream
)

type navState struct {
	selectedPane paneSelection
}

func (d *Demo) handleScreenEvent(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyUp:
			if d.nav.selectedPane == selectedResources {
				d.resourcesScroll().Up()
			} else {
				d.textScroll().Up()
			}
		case tcell.KeyDown:
			if d.nav.selectedPane == selectedResources {
				d.resourcesScroll().Down()
			} else {
				d.textScroll().Down()
			}
		case tcell.KeyLeft:
			d.nav.selectedPane = selectedResources
		case tcell.KeyRight:
			d.nav.selectedPane = selectedStream
		case tcell.KeyRune:
			switch ev.Rune() {
			case 'q':
				return true
			}
		}

	}
	return false
}

func (d *Demo) textScroll() rty.TextScroller {
	return d.rty.TextScroller("stream")
}

func (d *Demo) resourcesScroll() rty.TextScroller {
	return d.rty.TextScroller("resources")
}
