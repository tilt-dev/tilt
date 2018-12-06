package rty

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell"
)

func TestModal(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	{
		bg := Bg(NewGrowingBox(), tcell.ColorRed)
		fg := Bg(NewGrowingBox(), tcell.ColorBlue)
		l := NewModalLayout(bg, fg, 0.8, true)
		i.Run("modal blue box on red box", 20, 20, l)
	}

	{
		bg := NewLines()
		for i := 0; i < 20; i++ {
			bg.Add(TextString(strings.Repeat("i", 20)))
		}

		fg := NewGrowingBox()
		fg.SetInner(TextString("hello world"))
		l := NewModalLayout(bg, fg, 0.5, true)
		i.Run("modal text on top of text", 10, 10, l)
	}

	{
		bg := Bg(NewGrowingBox(), tcell.ColorRed)
		fg := NewFixedSize(Bg(NewGrowingBox(), tcell.ColorBlue), 3, 3)
		l := NewModalLayout(bg, fg, 0.8, false)
		i.Run("non-fixed modal blue box on red box", 20, 20, l)
	}
}
