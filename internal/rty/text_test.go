package rty

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tcell"
)

func TestTextString(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	i.Run("one-line text string", 20, 1, TextString("hello world"))
	i.Run("two-line text string", 20, 2, TextString("hello\nworld"))
	i.Run("two-line text string in one-line container", 20, 1, TextString("hello\nworld"))
	i.Run("horizontally overflowed text string", 2, 1, TextString("hello world"))
	i.Run("vertically overflowed text string", 10, 10, TextString(strings.Repeat("hi\n", 20)))
}

func TestStyledText(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	i.Run("blue string", 20, 1, ColoredString("hello world", tcell.ColorBlue))
	i.Run("black on white string", 20, 1, BgColoredString("hello world", tcell.ColorBlack, tcell.ColorWhite))
	c := NewStringBuilder().Text("hello ").Fg(tcell.ColorBlue).Text("world").Build()
	i.Run("multi-color string", 20, 1, c)
}

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

func TestLines(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	fl := NewFlexLayout(DirHor)
	for j := 0; j < 10; j++ {
		fl.Add(TextString("x"))
	}
	err := i.runCaptureError("overflowed multi-string line", 3, 1, fl)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "can't render in 3 columns")
	}

	l := NewLines()
	l.Add(NewStringBuilder().Text("hello").Build())
	l.Add(NewStringBuilder().Text("hello").Text("goodbye").Build())
	i.Run("lines of stringbuilders", 10, 10, l)

	l = NewLines()
	line := NewLine()
	line.Add(TextString("hello"))
	l.Add(line)
	line = NewLine()
	line.Add(TextString("hello"))
	line.Add(TextString("goodbye"))
	l.Add(line)
	i.Run("lines of lines", 30, 10, l)

	l = NewLines()
	l.Add(TextString("the quick brown fox\njumped over the lazy dog"))
	l.Add(TextString("here is another line"))
	i.Run("wrapped line followed by another line", 10, 20, l)
}

func TestStringBuilder(t *testing.T) {
	sb := NewStringBuilder()
	sb.Text("hello")
	w, h := sb.Build().Size(10, 1)
	assert.Equal(t, 5, w)
	assert.Equal(t, 1, h)

	sb = NewStringBuilder()
	sb.Text("hello world\ngoodbye")
	w, h = sb.Build().Size(5, 10)
	assert.Equal(t, 5, w)
	assert.Equal(t, 5, h)
}

func TestANSICodes(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	sb := NewStringBuilder().Text("\x1b[31mhello \x1b[33mworld")
	i.Run("red hello yellow world", 20, 1, sb.Build())

	sb = NewStringBuilder().Text("\x1b[44mhello \x1bcworld")
	i.Run("blue-bg hello, default-bg world", 20, 1, sb.Build())
}
