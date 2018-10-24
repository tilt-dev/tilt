package rty

import (
	"strings"
	"testing"

	"github.com/windmilleng/tcell"
)

func TestModal(t *testing.T) {
	f := newLayoutTestFixture(t)
	defer f.cleanUp()

	{
		bg := Bg(NewBox(), tcell.ColorRed)
		fg := Bg(NewBox(), tcell.ColorBlue)
		l := NewModalLayout(bg, fg, 0.8)
		f.run("modal blue box on red box", 20, 20, l)
	}

	{
		bg := NewLines()
		for i := 0; i < 20; i++ {
			bg.Add(TextString(strings.Repeat("i", 20)))
		}

		fg := NewBox()
		fg.SetInner(TextString("hello world"))
		l := NewModalLayout(bg, fg, 0.5)
		f.run("modal text on top of text", 10, 10, l)
	}
}
