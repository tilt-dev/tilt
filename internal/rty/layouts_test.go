package rty

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tcell"
)

func TestFlexLayoutOverflow(t *testing.T) {
	sc := tcell.NewSimulationScreen("")
	err := sc.Init()
	assert.NoError(t, err)
	sc.SetSize(8, 1)

	r := NewRTY(sc)

	f := NewFlexLayout(DirHor)
	f.Add(TextString("hello"))
	f.Add(TextString("world"))
	err = r.Render(f)
	assert.NoError(t, err)

	i := NewInteractiveTester(t, screen)
	i.Run("text overflow", 8, 1, f)
}

func TestBoxes(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	i.Run("10x10 box", 10, 10, NewGrowingBox())
	b := NewGrowingBox()
	b.SetFocused(true)
	i.Run("focused box", 10, 10, b)
	b = NewGrowingBox()
	b.SetInner(TextString("hello world"))
	i.Run("text in box", 20, 10, b)
	i.Run("wrapped text in box", 10, 10, b)
	b.SetTitle("so very important")
	i.Run("box with title", 20, 10, b)
	i.Run("box with short title", 5, 10, b)

	b = NewBox(TextString("hello world"))
	i.Run("non-growing box", 20, 20, b)
}

func TestStyles(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	var c Component
	c = NewGrowingBox()
	c = Fg(c, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	i.Run("green on white box", 10, 10, c)

	b := NewGrowingBox()
	b.SetInner(TextString("hello world"))
	c = Fg(b, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	i.Run("green on white box with text inside", 10, 10, c)

	b = NewGrowingBox()
	b.SetInner(BgColoredString("hello world", tcell.ColorBlue, tcell.ColorGreen))
	c = Fg(b, tcell.ColorGreen)
	c = Bg(c, tcell.ColorWhite)
	i.Run("green on white box with blue on green text inside", 10, 10, c)

	l := NewFlexLayout(DirHor)
	l.Add(Bg(NewGrowingBox(), tcell.ColorBlue))
	l.Add(Bg(NewGrowingBox(), tcell.ColorWhite))
	l.Add(Bg(NewGrowingBox(), tcell.ColorRed))
	i.Run("blue, white, red boxes horizontally", 30, 30, l)

	l = NewFlexLayout(DirVert)
	l.Add(Bg(NewGrowingBox(), tcell.ColorBlue))
	l.Add(Bg(NewGrowingBox(), tcell.ColorWhite))
	l.Add(Bg(NewGrowingBox(), tcell.ColorRed))
	i.Run("blue, white, red boxes vertically", 30, 30, l)
}

func TestConcatLayout(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	cl := NewConcatLayout(DirVert)
	cl.Add(TextString("hello"))
	cl.Add(TextString("goodbye"))
	i.Run("two strings in ConcatLayout", 15, 15, cl)

	cl = NewConcatLayout(DirHor)
	cl.Add(TextString("HEADER"))
	cl.AddDynamic(TextString(strings.Repeat("helllllo", 20)))
	i.Run("wrapping on right of ConcatLayout", 20, 20, cl)
}

func TestAlignEnd(t *testing.T) {
	i := NewInteractiveTester(t, screen)
	l := NewMinLengthLayout(10, DirHor).
		SetAlign(AlignEnd).
		Add(TextString("hello"))
	i.Run("align right on min-length layout", 15, 15, NewBox(l))
}
