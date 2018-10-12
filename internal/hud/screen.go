package hud

import (
	"fmt"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/windmilleng/tcell"
)

type styledString struct {
	string
	style tcell.Style
}

type pen struct {
	s     tcell.Screen
	x     int
	y     int
	style tcell.Style
}

func newPen(s tcell.Screen) *pen {
	return &pen{
		s:     s,
		x:     0,
		y:     0,
		style: tcell.StyleDefault,
	}
}

// NOTE(maia): largely stolen from tcell demos, we may want to roll our own

func (p *pen) putlnf(format string, a ...interface{}) {
	p.putln(fmt.Sprintf(format, a...))
}

func (p *pen) putStyledString(strings ...styledString) {
	for _, s := range strings {
		p.x += puts(p.s, s.style, p.x, p.y, s.string)
	}
}

func (p *pen) putlnStyledString(s ...styledString) {
	p.putStyledString(s...)
	p.newln()
}

func (p *pen) putln(str string) {
	// p.x = 0
	p.puts(str)
	// p.y++
	p.newln()
}

func (p *pen) newln() {
	p.x = 0
	p.y++
}

func (p *pen) putsf(format string, a ...interface{}) {
	p.puts(fmt.Sprintf(format, a...))
}

func (p *pen) puts(str string) {
	p.x += puts(p.s, p.style, p.x, p.y, str)
}

func puts(s tcell.Screen, style tcell.Style, x, y int, str string) int {
	i := 0
	var deferred []rune
	dwidth := 0
	zwj := false
	for _, r := range str {
		if r == '\u200d' {
			if len(deferred) == 0 {
				deferred = append(deferred, ' ')
				dwidth = 1
			}
			deferred = append(deferred, r)
			zwj = true
			continue
		}
		if zwj {
			deferred = append(deferred, r)
			zwj = false
			continue
		}
		switch runewidth.RuneWidth(r) {
		case 0:
			if len(deferred) == 0 {
				deferred = append(deferred, ' ')
				dwidth = 1
			}
		case 1:
			if len(deferred) != 0 {
				s.SetContent(x+i, y, deferred[0], deferred[1:], style)
				i += dwidth
			}
			deferred = nil
			dwidth = 1
		case 2:
			if len(deferred) != 0 {
				s.SetContent(x+i, y, deferred[0], deferred[1:], style)
				i += dwidth
			}
			deferred = nil
			dwidth = 2
		}
		deferred = append(deferred, r)
	}
	if len(deferred) != 0 {
		s.SetContent(x+i, y, deferred[0], deferred[1:], style)
		i += dwidth
	}
	return i
}
