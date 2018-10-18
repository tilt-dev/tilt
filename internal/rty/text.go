package rty

import (
	"fmt"

	"github.com/windmilleng/tcell"
)

type StringBuilder interface {
	Text(string) StringBuilder
	Textf(string, ...interface{}) StringBuilder
	Fg(tcell.Color) StringBuilder
	Bg(tcell.Color) StringBuilder
	Build() Component
}

func NewStringBuilder() StringBuilder {
	return &stringBuilder{}
}

type directive interface {
	directive()
}

type textDirective string
type fgDirective tcell.Color
type bgDirective tcell.Color

func (textDirective) directive() {}
func (fgDirective) directive()   {}
func (bgDirective) directive()   {}

type stringBuilder struct {
	directives []directive
}

func (b *stringBuilder) Text(t string) StringBuilder {
	b.directives = append(b.directives, textDirective(t))
	return b
}

func (b *stringBuilder) Textf(format string, a ...interface{}) StringBuilder {
	b.directives = append(b.directives, textDirective(fmt.Sprintf(format, a...)))
	return b
}

func (b *stringBuilder) Fg(c tcell.Color) StringBuilder {
	b.directives = append(b.directives, fgDirective(c))
	return b
}

func (b *stringBuilder) Bg(c tcell.Color) StringBuilder {
	b.directives = append(b.directives, bgDirective(c))
	return b
}

func (b *stringBuilder) Build() Component {
	return &StringLayout{directives: b.directives}
}

type StringLayout struct {
	directives []directive
}

func TextString(s string) Component {
	return NewStringBuilder().Text(s).Build()
}

func ColoredString(s string, fg tcell.Color) Component {
	return NewStringBuilder().Fg(fg).Text(s).Build()
}

func BgColoredString(s string, fg tcell.Color, bg tcell.Color) Component {
	return NewStringBuilder().Fg(fg).Bg(bg).Text(s).Build()
}

func (l *StringLayout) Size(availWidth int, availHeight int) (int, int) {
	return l.render(nil, availWidth, availHeight)
}

func (l *StringLayout) Render(w Writer, width int, height int) error {
	l.render(w, width, height)
	return nil
}

// returns width, height for laying out full string
func (l *StringLayout) render(w Writer, width int, height int) (int, int) {
	nextX, nextY := 0, 0
	maxWidth := 0
	for _, d := range l.directives {
		var s string
		switch d := d.(type) {
		case textDirective:
			s = string(d)
		case fgDirective:
			if w != nil {
				w = w.Foreground(tcell.Color(d))
			}
			continue
		case bgDirective:
			if w != nil {
				w = w.Background(tcell.Color(d))
			}
			continue
		default:
			panic(fmt.Errorf("StringLayout.Render: unexpected directive %T %+v", d, d))
		}
		// now we know it's a text directive
		for _, ch := range s {
			// TODO(dbentley): combining characters
			// TODO(dbentley): tab, etc.
			// TODO(dbentley): runewidth
			if nextX >= width {
				nextX, nextY = 0, nextY+1
			}
			if nextX+1 > maxWidth {
				maxWidth = nextX + 1
			}
			if nextY >= height {
				return maxWidth, height
			}
			if ch == '\n' {
				if nextX == 0 && w != nil {
					// maked sure we take up our space
					w.SetContent(nextY, nextY, ch, nil)
				}
				nextX, nextY = 0, nextY+1
				continue
			}

			if w != nil {
				w.SetContent(nextX, nextY, ch, nil)
			}
			nextX = nextX + 1
		}
	}
	if nextY == 0 {
		nextY = 1
	}
	return maxWidth, nextY
}
