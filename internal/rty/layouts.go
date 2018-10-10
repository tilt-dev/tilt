package rty

import (
	"fmt"

	"github.com/windmilleng/tcell"
)

// Layouts implement Component

// FlexLayout lays out its sub-components.
type FlexLayout struct {
	dir Dir
	cs  []Component
}

func NewFlexLayout(dir Dir) *FlexLayout {
	return &FlexLayout{
		dir: dir,
	}
}

func (l *FlexLayout) Add(c Component) {
	l.cs = append(l.cs, c)
}

func (l *FlexLayout) Render(c CanvasWriter) error {
	width, height := c.Size()

	length := width
	if l.dir == DirVert {
		length = height
	}

	allocated := 0
	numFlex := len(l.cs)

	for _, c := range l.cs {
		fdc, ok := c.(FixedDimComponent)
		if !ok {
			continue
		}
		numFlex--
		allocated += fdc.FixedDimSize()
	}

	flexTotal := length - allocated
	if flexTotal < 0 {
		return fmt.Errorf("FlexLayout can't render in %v lines; need at least %v", length, allocated)
	}
	var flexLengths []int
	for i := 0; i < numFlex; i++ {
		elemLength := flexTotal / numFlex
		flexLengths = append(flexLengths, elemLength)
		flexTotal -= elemLength
	}

	offset := 0
	for _, comp := range l.cs {
		elemLength := 0
		fdcomp, ok := comp.(FixedDimComponent)
		if ok {
			elemLength = fdcomp.FixedDimSize()
		} else {
			elemLength, flexLengths = flexLengths[0], flexLengths[1:]
		}

		var subC CanvasWriter

		if l.dir == DirHor {
			subC = c.Sub(offset, 0, elemLength, height)
		} else {
			subC = c.Sub(0, offset, width, elemLength)
		}

		offset += elemLength

		if err := comp.Render(subC); err != nil {
			return err
		}
	}

	return nil
}

type ScrollLayout struct {
	concat *ConcatLayout
}

func NewScrollLayout(dir Dir) *ScrollLayout {
	if dir == DirHor {
		panic(fmt.Errorf("ScrollLayout doesn't support Horizontal"))
	}

	return &ScrollLayout{
		concat: NewConcatLayout(DirVert),
	}
}

func (l *ScrollLayout) Add(c FixedDimComponent) {
	l.concat.Add(c)
}

func (l *ScrollLayout) Render(c CanvasWriter) error {
	width, height := c.Size()
	if height < l.concat.FixedDimSize() {
		height = l.concat.FixedDimSize()
	}
	tempC := NewTempCanvas(width, height)
	if err := l.concat.Render(tempC); err != nil {
		return err
	}

	Copy(tempC, c)
	return nil
}

type ConcatLayout struct {
	dir Dir
	cs  []FixedDimComponent
}

func NewConcatLayout(dir Dir) *ConcatLayout {
	return &ConcatLayout{dir: dir}
}

func (l *ConcatLayout) Add(c FixedDimComponent) {
	l.cs = append(l.cs, c)
}

func (l *ConcatLayout) FixedDimSize() int {
	r := 0
	for _, c := range l.cs {
		r += c.FixedDimSize()
	}

	return r
}

func (l *ConcatLayout) Render(c CanvasWriter) error {
	width, height := c.Size()
	offset := 0
	for _, subL := range l.cs {
		elemLength := subL.FixedDimSize()

		var subC CanvasWriter
		if l.dir == DirHor {
			subC = c.Sub(offset, 0, elemLength, height)
		} else {
			subC = c.Sub(0, offset, width, elemLength)
		}

		if err := subL.Render(subC); err != nil {
			return err
		}
		offset += elemLength
	}
	return nil
}

func NewLines() *ConcatLayout {
	return NewConcatLayout(DirVert)
}

type Line struct {
	del *FlexLayout
}

func NewLine() *Line {
	return &Line{del: NewFlexLayout(DirHor)}
}

func (l *Line) Add(c Component) {
	l.del.Add(c)
}

func (l *Line) FixedDimSize() int {
	return 1
}

func (l *Line) Render(c CanvasWriter) error {
	return l.del.Render(c)
}

type Box struct {
	inner Component
}

func NewBox() *Box {
	return &Box{}
}

func (b *Box) SetInner(c Component) {
	b.inner = c
}

func (b *Box) Render(c CanvasWriter) error {
	width, height := c.Size()

	for i := 0; i < width; i++ {
		c.SetContent(i, 0, '+', nil, tcell.StyleDefault)
		c.SetContent(i, height-1, '+', nil, tcell.StyleDefault)
	}

	for i := 0; i < height; i++ {
		c.SetContent(0, i, '+', nil, tcell.StyleDefault)
		c.SetContent(width-1, i, '+', nil, tcell.StyleDefault)
	}

	if b.inner == nil {
		return nil
	}

	return b.inner.Render(c.Sub(1, 1, width-2, height-2))
}

// FixedDimSizeLayout fixes a component to a size
type FixedDimSizeLayout struct {
	del    Component
	length int
}

func NewFixedDimSize(del Component, length int) *FixedDimSizeLayout {
	return &FixedDimSizeLayout{del: del, length: length}
}

func (l *FixedDimSizeLayout) FixedDimSize() int {
	return l.length
}

func (l *FixedDimSizeLayout) Render(c CanvasWriter) error {
	return l.del.Render(c)
}
