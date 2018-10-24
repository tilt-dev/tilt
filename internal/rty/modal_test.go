package rty

import (
	"strings"
	"testing"

	"github.com/windmilleng/tcell"
)

func TestModal(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	{
		bg := Bg(NewBox(), tcell.ColorRed)
		fg := Bg(NewBox(), tcell.ColorBlue)
		l := NewModalLayout(bg, fg, 0.8)
		i.Run("modal blue box on red box", 20, 20, l)
	}

	{
		bg := NewLines()
		for i := 0; i < 20; i++ {
			bg.Add(TextString(strings.Repeat("i", 20)))
		}

		fg := NewBox()
		fg.SetInner(TextString("hello world"))
		l := NewModalLayout(bg, fg, 0.5)
		i.Run("modal text on top of text", 10, 10, l)
	}
}
