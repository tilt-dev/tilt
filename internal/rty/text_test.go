package rty

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gdamore/tcell"
)

func TestTextString(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	i.Run("one-line text string", 20, 1, TextString("hello world"))
	i.Run("two-line text string", 20, 2, TextString("hello\nworld"))
	i.Run("two-line text string in one-line container", 20, 1, TextString("hello\nworld"))
	i.Run("horizontally overflowed text string", 2, 1, TextString("hello world"))
	i.Run("horizontally overflowed long text string", 20, 2, TextString(strings.Repeat("hello", 20)))
	i.Run("vertically overflowed via newlines text string", 10, 10, TextString(strings.Repeat("hi\n", 20)))
	i.Run("vertically overflowed via wrap text string", 5, 5, TextString(strings.Repeat("xxxxxxxxxx\n", 200)))
}

func TestStyledText(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	i.Run("blue string", 20, 1, ColoredString("hello world", tcell.ColorBlue))
	i.Run("black on white string", 20, 1, BgColoredString("hello world", tcell.ColorBlack, tcell.ColorWhite))
	c := NewStringBuilder().Text("hello ").Fg(tcell.ColorBlue).Text("world").Build()
	i.Run("multi-color string", 20, 1, c)
}

func TestLines(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	fl := NewFlexLayout(DirHor)
	for j := 0; j < 10; j++ {
		fl.Add(TextString("x"))
	}
	i.Run("overflowed multi-string line", 3, 1, fl)

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
	w, h, err := sb.Build().Size(10, 1)
	assert.NoError(t, err)
	assert.Equal(t, 5, w)
	assert.Equal(t, 1, h)

	sb = NewStringBuilder()
	sb.Text("hello world\ngoodbye")
	w, h, err = sb.Build().Size(5, 10)
	assert.NoError(t, err)
	assert.Equal(t, 5, w)
	assert.Equal(t, 6, h)

	sb = NewStringBuilder()
	sb.Text("hello world")
	w, h, err = sb.Build().Size(3, 3)
	assert.NoError(t, err)
	assert.Equal(t, 3, w)
	assert.Equal(t, 3, h)
}

func TestANSICodes(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	sb := NewStringBuilder().Text("\x1b[31mhello \x1b[33mworld")
	i.Run("red hello yellow world", 20, 1, sb.Build())

	sb = NewStringBuilder().Text("\x1b[44mhello \x1bcworld")
	i.Run("blue-bg hello, default-bg world", 20, 1, sb.Build())
}
