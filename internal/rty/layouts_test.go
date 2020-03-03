package rty

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell"
	"github.com/stretchr/testify/assert"
)

func TestFlexLayoutOverflow(t *testing.T) {
	sc := tcell.NewSimulationScreen("")
	err := sc.Init()
	assert.NoError(t, err)

	r := NewRTY(sc, t)

	f := NewFlexLayout(DirHor)
	f.Add(TextString("hello"))
	f.Add(TextString("world"))
	r.Render(f)

	i := NewInteractiveTester(t, screen)
	i.Run("text overflow", 8, 1, f)
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

func TestNestedConcatLayoutOverflow(t *testing.T) {
	sc := tcell.NewSimulationScreen("")
	err := sc.Init()
	assert.NoError(t, err)

	r := NewRTY(sc, t)

	f1 := NewConcatLayout(DirHor)
	for i := 0; i < 10; i++ {
		f1.Add(TextString("x"))
		f1.AddDynamic(NewFillerString(' '))
	}

	f2 := NewConcatLayout(DirHor)
	f1.Add(f2)
	for i := 0; i < 10; i++ {
		f2.Add(TextString("y"))
		f2.AddDynamic(NewFillerString(' '))
	}

	r.Render(NewFixedSize(f1, 8, 1))

	i := NewInteractiveTester(t, screen)
	i.Run("concat text overflow", 8, 1, f1)
}

func TestMinWidthInNestedConcatLayoutOverflow(t *testing.T) {
	sc := tcell.NewSimulationScreen("")
	err := sc.Init()
	assert.NoError(t, err)

	r := NewRTY(sc, t)

	f1 := NewConcatLayout(DirHor)
	for i := 0; i < 10; i++ {
		f1.Add(NewMinLengthLayout(3, DirHor).Add(TextString("x")))
		f1.AddDynamic(NewFillerString(' '))
	}

	f2 := NewConcatLayout(DirHor)
	f2.Add(f1)

	r.Render(f2)

	i := NewInteractiveTester(t, screen)
	i.Run("min width concat text overflow", 8, 1, f2)
}

func TestTailLayout(t *testing.T) {
	sc := tcell.NewSimulationScreen("")
	err := sc.Init()
	assert.NoError(t, err)

	f := NewConcatLayout(DirVert)
	for i := 0; i < 15; i++ {
		f.Add(TextString(fmt.Sprintf("line %d text text", i)))
	}

	tail := NewBox(NewTailLayout(f))
	i := NewInteractiveTester(t, screen)
	i.Run("tail layout no overflow", 20, 20, tail)
	i.Run("tail layout overflow-y", 20, 10, tail)
	i.Run("tail layout overflow-x", 15, 40, tail)
	i.Run("tail layout overflow-xy", 15, 10, tail)
}

func TestMaxLengthLayout(t *testing.T) {
	sc := tcell.NewSimulationScreen("")
	err := sc.Init()
	assert.NoError(t, err)

	f := NewConcatLayout(DirVert)
	for i := 0; i < 15; i++ {
		f.Add(TextString(fmt.Sprintf("line %d text text", i)))
	}

	box := NewBox(NewMaxLengthLayout(f, DirVert, 20))
	i := NewInteractiveTester(t, screen)
	i.Run("max layout no overflow", 20, 20, box)
	i.Run("max layout overflow", 15, 40, box)
}
