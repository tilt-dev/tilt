package rty

import (
	"github.com/windmilleng/tcell"
)

func SimpleTextCases(f *fixture) {
	f.add(20, 1, TextString("hello world"))
	f.add(20, 2, TextString("hello\nworld"))
	f.add(20, 1, TextString("hello\nworld"))
}

func StyledTextCases(f *fixture) {
	f.add(20, 1, ColoredString("hello world", tcell.ColorBlue))
	f.add(20, 1, BgColoredString("hello world", tcell.ColorBlack, tcell.ColorWhite))
	c := NewStringBuilder().Text("hello ").Fg(tcell.ColorBlue).Text("world").Build()
	f.add(20, 1, c)
}

func BoxCases(f *fixture) {
	f.add(10, 10, NewBox())
	b := NewBox()
	b.SetFocused(true)
	f.add(10, 10, b)
	b = NewBox()
	b.SetInner(TextString("hello world"))
	f.add(20, 10, b)
	f.add(10, 10, b)
}

func StyleCases(f *fixture) {
	var c Component
	c = NewBox()
	c = Fg(c, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	f.add(10, 10, c)
	b := NewBox()
	b.SetInner(TextString("hello world"))
	c = Fg(b, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	f.add(10, 10, c)
	b = NewBox()
	b.SetInner(BgColoredString("hello world", tcell.ColorBlue, tcell.ColorGreen))
	c = Fg(b, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	f.add(10, 10, c)

	l := NewFlexLayout(DirHor)
	l.Add(Bg(NewBox(), tcell.ColorBlue))
	l.Add(Bg(NewBox(), tcell.ColorWhite))
	l.Add(Bg(NewBox(), tcell.ColorRed))
	f.add(30, 30, l)

	l = NewFlexLayout(DirVert)
	l.Add(Bg(NewBox(), tcell.ColorBlue))
	l.Add(Bg(NewBox(), tcell.ColorWhite))
	l.Add(Bg(NewBox(), tcell.ColorRed))
	f.add(30, 30, l)
}
