package rty

import (
	"fmt"

	"github.com/rivo/tview"

	"github.com/windmilleng/tcell"
)

// Layouts implement Component

type Dir int

const (
	DirHor Dir = iota
	DirVert
)

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

func whToLd(width int, height int, dir Dir) (length int, depth int) {
	if dir == DirVert {
		return height, width
	}
	return width, height
}

func ldToWh(length int, depth int, dir Dir) (width int, height int) {
	if dir == DirVert {
		return depth, length
	}
	return length, depth
}

func (l *FlexLayout) Size(width int, height int) (int, int) {
	return width, height
}

func (l *FlexLayout) Render(w Writer, width, height int) error {
	length, _ := whToLd(width, height, l.dir)

	allocations := make([]int, len(l.cs))
	allocated := 0
	var flexIdxs []int

	for i, c := range l.cs {
		reqWidth, reqHeight := c.Size(width, height)
		reqLen, _ := whToLd(reqWidth, reqHeight, l.dir)
		if reqLen >= length {
			flexIdxs = append(flexIdxs, i)
		} else {
			allocations[i] = reqLen
			allocated += reqLen
		}
	}

	flexTotal := length - allocated
	if flexTotal < 0 {
		noun := "lines"
		if l.dir == DirHor {
			noun = "columns"
		}
		return fmt.Errorf("FlexLayout can't render in %v %s; need at least %v", length, noun, allocated)
	}
	numFlex := len(flexIdxs)
	for _, i := range flexIdxs {
		elemLength := flexTotal / numFlex
		allocations[i] = elemLength
		numFlex--
		flexTotal -= elemLength
	}

	offset := 0
	for i, c := range l.cs {
		elemLength := allocations[i]

		var subW Writer

		if l.dir == DirHor {
			subW = w.Divide(offset, 0, allocations[i], height)
		} else {
			subW = w.Divide(0, offset, width, allocations[i])
		}

		offset += elemLength

		subW.RenderChild(c)
	}
	return nil
}

type ConcatLayout struct {
	dir Dir
	cs  []Component
}

func NewConcatLayout(dir Dir) *ConcatLayout {
	return &ConcatLayout{dir: dir}
}

func (l *ConcatLayout) Add(c Component) {
	l.cs = append(l.cs, c)
}

func (l *ConcatLayout) Size(width, height int) (int, int) {
	totalLen := 0
	maxDepth := 0
	for _, c := range l.cs {
		reqWidth, reqHeight := c.Size(width, height)
		reqLen, reqDepth := whToLd(reqWidth, reqHeight, l.dir)
		if reqLen == GROW {
			totalLen = GROW
		}
		if totalLen != GROW {
			totalLen += reqLen
		}
		if reqDepth > maxDepth {
			maxDepth = reqDepth
		}
	}
	return ldToWh(totalLen, maxDepth, l.dir)
}

func (l *ConcatLayout) Render(w Writer, width int, height int) error {
	offset := 0
	for _, c := range l.cs {
		reqWidth, reqHeight := c.Size(width, height)

		var subW Writer
		if l.dir == DirHor {
			subW = w.Divide(offset, 0, reqWidth, reqHeight)
			offset += reqWidth
		} else {
			subW = w.Divide(0, offset, reqWidth, reqHeight)
			offset += reqHeight
		}

		subW.RenderChild(c)
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

func (l *Line) Size(width int, height int) (int, int) {
	return width, 1
}

func (l *Line) Render(w Writer, width int, height int) error {
	w.SetContent(0, 0, 0, nil) // set at least one to take up our line
	w.Divide(0, 0, width, height).RenderChild(l.del)
	return nil
}

type ColorLayout struct {
	del        Component
	color      tcell.Color
	foreground bool
}

func Fg(del Component, color tcell.Color) Component {
	return &ColorLayout{
		del:        del,
		color:      color,
		foreground: true,
	}
}

func Bg(del Component, color tcell.Color) Component {
	return &ColorLayout{
		del:        del,
		color:      color,
		foreground: false,
	}
}

func (l *ColorLayout) Size(width int, height int) (int, int) {
	return l.del.Size(width, height)
}

func (l *ColorLayout) Render(w Writer, width int, height int) error {
	if l.foreground {
		w = w.Foreground(l.color)
	} else {
		w = w.Background(l.color)
	}
	w = w.Fill()
	w.RenderChild(l.del)
	return nil
}

type Box struct {
	focused bool
	inner   Component
}

func NewBox() *Box {
	return &Box{}
}

func (b *Box) SetInner(c Component) {
	b.inner = c
}

func (b *Box) SetFocused(focused bool) {
	b.focused = focused
}

func (b *Box) Size(width int, height int) (int, int) {
	return width, height
}

func (b *Box) Render(w Writer, width int, height int) error {
	if height == GROW && b.inner == nil {
		return fmt.Errorf("box must have either fixed height or a child")
	}

	if b.inner != nil {
		innerHeight := height - 2
		if height == GROW {
			innerHeight = GROW
		}

		childHeight := w.Divide(1, 1, width-2, innerHeight).RenderChild(b.inner)
		height = childHeight + 2
	}

	hor := tview.BoxDrawingsLightHorizontal
	vert := tview.BoxDrawingsLightVertical
	tl := tview.BoxDrawingsLightDownAndRight
	tr := tview.BoxDrawingsLightDownAndLeft
	bl := tview.BoxDrawingsLightUpAndRight
	br := tview.BoxDrawingsLightUpAndLeft
	if b.focused {
		hor = tview.BoxDrawingsDoubleHorizontal
		vert = tview.BoxDrawingsDoubleVertical
		tl = tview.BoxDrawingsDoubleDownAndRight
		tr = tview.BoxDrawingsDoubleDownAndLeft
		bl = tview.BoxDrawingsDoubleUpAndRight
		br = tview.BoxDrawingsDoubleUpAndLeft
	}

	for i := 1; i < width-1; i++ {
		w.SetContent(i, 0, hor, nil)
		w.SetContent(i, height-1, hor, nil)
	}

	for i := 1; i < height-1; i++ {
		w.SetContent(0, i, vert, nil)
		w.SetContent(width-1, i, vert, nil)
	}

	w.SetContent(0, 0, tl, nil)
	w.SetContent(width-1, 0, tr, nil)
	w.SetContent(0, height-1, bl, nil)
	w.SetContent(width-1, height-1, br, nil)

	return nil
}

// FixedSizeLayout fixes a component to a size
type FixedSizeLayout struct {
	del    Component
	width  int
	height int
}

func NewFixedSize(del Component, width int, height int) *FixedSizeLayout {
	return &FixedSizeLayout{del: del, width: width, height: height}
}

func (l *FixedSizeLayout) Size(width int, height int) (int, int) {
	if l.width != GROW && l.height != GROW {
		return l.width, l.height
	}
	rWidth, rHeight := l.width, l.height
	delWidth, delHeight := l.del.Size(width, height)
	if rWidth == GROW {
		rWidth = delWidth
	}
	if rHeight == GROW {
		rHeight = delHeight
	}

	return rWidth, rHeight
}

func (l *FixedSizeLayout) Render(w Writer, width int, height int) error {
	w.RenderChild(l.del)
	return nil
}

type ModalLayout struct {
	bg       Component
	fg       Component
	fraction float64
}

// fg will be rendered on top of bg, using fraction/1 of the height and width of the screen
func NewModalLayout(bg Component, fg Component, fraction float64) *ModalLayout {
	return &ModalLayout{fg: fg, bg: bg, fraction: fraction}
}

func (l *ModalLayout) Size(width int, height int) (int, int) {
	w, h := l.bg.Size(width, height)
	fgw, fgh := l.fg.Size(width, height)
	if fgw > w {
		w = fgw
	}
	if fgh > h {
		h = fgh
	}

	return w, h
}

func (l *ModalLayout) Render(w Writer, width int, height int) error {
	w.RenderChild(l.bg)

	f := (1 - l.fraction) / 2
	mx := int(f * float64(width))
	my := int(f * float64(height))
	mh := int((1 - 2*f) * float64(width))
	mw := int((1 - 2*f) * float64(height))
	w = w.Divide(mx, my, mh, mw)
	w.RenderChild(l.fg)
	return nil
}
