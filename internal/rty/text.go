package rty

import (
	"fmt"

	"github.com/windmilleng/tcell"
)

// TODO(dbentley): this whole file should print smarter (and strip out control characters)

// Layout a string on one line, suitable for a label
type StringLayout struct {
	s string
}

func String(s string) *StringLayout {
	return &StringLayout{s: s}
}

func (l *StringLayout) FixedDimSize() int {
	return len(l.s)
}

func (l *StringLayout) Render(c CanvasWriter) error {
	printStringOneLine(c, l.s)
	return nil
}

// Fills a space by repeating a string
type FillerString struct {
	ch rune
}

func NewFillerString(ch rune) *FillerString {
	return &FillerString{ch: ch}
}

func (l *FillerString) Render(c CanvasWriter) error {
	width, _ := c.Size()
	for i := 0; i < width; i++ {
		c.SetContent(i, 0, l.ch, nil, tcell.StyleDefault)
	}
	return nil
}

type TruncatingStrings struct {
	data []string
}

func NewTruncatingStrings(data []string) *TruncatingStrings {
	return &TruncatingStrings{data: data}
}

func (l *TruncatingStrings) Render(c CanvasWriter) error {
	width, _ := c.Size()

	c.SetContent(0, 0, '[', nil, tcell.StyleDefault)
	offset := 2 // "[ "
	for i, s := range l.data {
		subC := c.Sub(offset, 0, width-offset, 1)
		endText := fmt.Sprintf("and %d more ]", len(l.data)-i)
		if offset+len(endText)+len(s) > width {
			// ran out of space; truncate!
			printStringOneLine(subC, endText)
			return nil
		}
		printStringOneLine(subC, s+" ")
		offset += len(s) + 1
	}

	printStringOneLine(c.Sub(offset, 0, width-offset, 1), "]")
	return nil
}

func printStringOneLine(c CanvasWriter, s string) {
	for i, ch := range s {
		c.SetContent(i, 0, ch, nil, tcell.StyleDefault)
	}
}
