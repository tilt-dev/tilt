package rty

import (
	"testing"

	"github.com/windmilleng/tcell"
)

func TestTextString(t *testing.T) {
	f := newLayoutTestFixture(t)
	defer f.cleanUp()

	f.run("one-line text string", 20, 1, TextString("hello world"))
	f.run("two-line text string", 20, 2, TextString("hello\nworld"))
	f.run("two-line text string in one-line container", 20, 1, TextString("hello\nworld"))
	f.run("overflowed text string", 2, 1, TextString("hello world"))
}

func TestStyledText(t *testing.T) {
	f := newLayoutTestFixture(t)
	defer f.cleanUp()

	f.run("blue string", 20, 1, ColoredString("hello world", tcell.ColorBlue))
	f.run("black on white string", 20, 1, BgColoredString("hello world", tcell.ColorBlack, tcell.ColorWhite))
	c := NewStringBuilder().Text("hello ").Fg(tcell.ColorBlue).Text("world").Build()
	f.run("multi-color string", 20, 1, c)
}

func TestBoxes(t *testing.T) {
	f := newLayoutTestFixture(t)
	defer f.cleanUp()

	f.run("10x10 box", 10, 10, NewBox())
	b := NewBox()
	b.SetFocused(true)
	f.run("focused box", 10, 10, b)
	b = NewBox()
	b.SetInner(TextString("hello world"))
	f.run("text in box", 20, 10, b)
	f.run("overflowed text in box", 10, 10, b)
}

func TestStyles(t *testing.T) {
	f := newLayoutTestFixture(t)
	defer f.cleanUp()

	var c Component
	c = NewBox()
	c = Fg(c, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	f.run("green on white box", 10, 10, c)
	b := NewBox()
	b.SetInner(TextString("hello world"))
	c = Fg(b, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	f.run("green on white box with text inside", 10, 10, c)
	b = NewBox()
	b.SetInner(BgColoredString("hello world", tcell.ColorBlue, tcell.ColorGreen))
	c = Fg(b, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	f.run("green on white box with blue on green text inside", 10, 10, c)

	l := NewFlexLayout(DirHor)
	l.Add(Bg(NewBox(), tcell.ColorBlue))
	l.Add(Bg(NewBox(), tcell.ColorWhite))
	l.Add(Bg(NewBox(), tcell.ColorRed))
	f.run("blue, white, red boxes horizontally", 30, 30, l)

	l = NewFlexLayout(DirVert)
	l.Add(Bg(NewBox(), tcell.ColorBlue))
	l.Add(Bg(NewBox(), tcell.ColorWhite))
	l.Add(Bg(NewBox(), tcell.ColorRed))
	f.run("blue, white, red boxes vertically", 30, 30, l)
}
