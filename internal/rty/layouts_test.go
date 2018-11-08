package rty

import (
	"testing"

	"github.com/windmilleng/tcell"
)

func TestBoxes(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	i.Run("10x10 box", 10, 10, NewBox())
	b := NewBox()
	b.SetFocused(true)
	i.Run("focused box", 10, 10, b)
	b = NewBox()
	b.SetInner(TextString("hello world"))
	i.Run("text in box", 20, 10, b)
	i.Run("wrapped text in box", 10, 10, b)
	b.SetTitle("so very important")
	i.Run("box with title", 20, 10, b)
	i.Run("box with short title", 5, 10, b)
}

func TestStyles(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	var c Component
	c = NewBox()
	c = Fg(c, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	i.Run("green on white box", 10, 10, c)

	b := NewBox()
	b.SetInner(TextString("hello world"))
	c = Fg(b, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	i.Run("green on white box with text inside", 10, 10, c)

	b = NewBox()
	b.SetInner(BgColoredString("hello world", tcell.ColorBlue, tcell.ColorGreen))
	c = Fg(b, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	i.Run("green on white box with blue on green text inside", 10, 10, c)

	l := NewFlexLayout(DirHor)
	l.Add(Bg(NewBox(), tcell.ColorBlue))
	l.Add(Bg(NewBox(), tcell.ColorWhite))
	l.Add(Bg(NewBox(), tcell.ColorRed))
	i.Run("blue, white, red boxes horizontally", 30, 30, l)

	l = NewFlexLayout(DirVert)
	l.Add(Bg(NewBox(), tcell.ColorBlue))
	l.Add(Bg(NewBox(), tcell.ColorWhite))
	l.Add(Bg(NewBox(), tcell.ColorRed))
	i.Run("blue, white, red boxes vertically", 30, 30, l)
}
