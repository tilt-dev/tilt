package rty

import (
	"fmt"

	"github.com/windmilleng/tcell"
)

// TODO(dbentley): this whole file should print smarter (and strip out control characters)

// Layout a string on one line, suitable for a label
type StringLayout struct {
	id ID
	s  string
}

func String(id ID, s string) *StringLayout {
	return &StringLayout{id: id, s: s}
}

func (l *StringLayout) ID() ID {
	return l.id
}

func (l *StringLayout) Size(width int, height int) (int, int) {
	return len(l.s), 1
}

func (l *StringLayout) Render(w Writer, width int, height int) error {
	printStringOneLine(w, l.s)
	return nil
}

// Fills a space by repeating a string
type FillerString struct {
	id ID
	ch rune
}

func NewFillerString(id ID, ch rune) *FillerString {
	return &FillerString{id: id, ch: ch}
}

func (l *FillerString) ID() ID {
	return l.id
}

func (l *FillerString) Size(width, height int) (int, int) {
	return width, 1
}

func (l *FillerString) Render(w Writer, width int, height int) error {
	for i := 0; i < width; i++ {
		w.SetContent(i, 0, l.ch, nil, tcell.StyleDefault)
	}
	return nil
}

type TruncatingStrings struct {
	id   ID
	data []string
}

func NewTruncatingStrings(id ID, data []string) *TruncatingStrings {
	return &TruncatingStrings{id: id, data: data}
}

func (l *TruncatingStrings) ID() ID {
	return l.id
}

func (l *TruncatingStrings) Size(width int, height int) (int, int) {
	return width, height
}

func (l *TruncatingStrings) Render(w Writer, width int, height int) error {
	w.SetContent(0, 0, '[', nil, tcell.StyleDefault)
	offset := 2 // "[ "
	for i, s := range l.data {
		subW := w.Divide(offset, 0, width-offset, 1)
		endText := fmt.Sprintf("and %d more ]", len(l.data)-i)
		if offset+len(endText)+len(s) > width {
			// ran out of space; truncate!
			printStringOneLine(subW, endText)
			return nil
		}
		printStringOneLine(subW, s+" ")
		offset += len(s) + 1
	}

	printStringOneLine(w.Divide(offset, 0, width-offset, 1), "]")
	return nil
}

func printStringOneLine(w Writer, s string) {
	for i, ch := range s {
		w.SetContent(i, 0, ch, nil, tcell.StyleDefault)
	}
}

type WrappingTextLine struct {
	id   ID
	text string
}

func NewWrappingTextLine(id ID, text string) *WrappingTextLine {
	return &WrappingTextLine{
		id:   id,
		text: text,
	}
}

func (l *WrappingTextLine) ID() ID {
	return l.id
}

func (l *WrappingTextLine) Size(width int, height int) (int, int) {
	if len(l.text) == 0 {
		return width, 1
	}

	desiredHeight := len(l.text) / width
	if desiredHeight > height {
		// we'll make do
		return width, height
	}

	return width, desiredHeight
}

func (l *WrappingTextLine) Render(w Writer, width int, height int) error {
	x, y := 0, 0
	for _, ch := range l.text {
		w.SetContent(x, y, ch, nil, tcell.StyleDefault)
		x++
		if x == width {
			x = 0
			y++
			if y == height {
				break
			}
		}
	}
	return nil
}
