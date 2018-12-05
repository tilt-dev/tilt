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

type Align int

const (
	AlignStart Align = iota
	AlignEnd
)

// FlexLayout lays out its sub-components.
type FlexLayout struct {
	dir Dir
	cs  []Component
}

var _ Component = &FlexLayout{}

func NewFlexLayout(dir Dir) *FlexLayout {
	return &FlexLayout{
		dir: dir,
	}
}

func (l *FlexLayout) Add(c Component) *FlexLayout {
	l.cs = append(l.cs, c)
	return l
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

func (l *FlexLayout) Size(width int, height int) (int, int, error) {
	return width, height, nil
}

func (l *FlexLayout) Render(w Writer, width, height int) error {
	length, _ := whToLd(width, height, l.dir)

	allocations := make([]int, len(l.cs))
	allocated := 0
	var flexIdxs []int

	for i, c := range l.cs {
		reqWidth, reqHeight, err := c.Size(width, height)
		if err != nil {
			return err
		}
		reqLen, _ := whToLd(reqWidth, reqHeight, l.dir)
		if allocated+reqLen >= length {
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
			var err error
			subW, err = w.Divide(offset, 0, allocations[i], height)
			if err != nil {
				return err
			}
		} else {
			var err error
			subW, err = w.Divide(0, offset, width, allocations[i])
			if err != nil {
				return err
			}
		}

		offset += elemLength

		subW.RenderChild(c)
	}
	return nil
}

type concatLayoutComponent struct {
	c     Component
	fixed bool
}

type ConcatLayout struct {
	dir Dir
	cs  []concatLayoutComponent
}

var _ Component = &ConcatLayout{}

func NewConcatLayout(dir Dir) *ConcatLayout {
	return &ConcatLayout{dir: dir}
}

func (l *ConcatLayout) Add(c Component) *ConcatLayout {
	l.cs = append(l.cs, concatLayoutComponent{c, true})
	return l
}

// A ConcatLayout element can be either fixed or dynamic. Fixed components are all given a chance at the full
// canvas. If they ask for too much in sum, things will break.
// Dynamic components get equal shares of whatever is left after the fixed components get theirs.
// NB: There is currently a bit of a murky line between ConcatLayout and FlexLayout.
func (l *ConcatLayout) AddDynamic(c Component) *ConcatLayout {
	l.cs = append(l.cs, concatLayoutComponent{c, false})
	return l
}

func (l *ConcatLayout) allocate(width, height int) (widths []int, heights []int, allocatedLen int, maxDepth int, err error) {
	length, depth := whToLd(width, height, l.dir)

	type componentAndIndex struct {
		c     Component
		index int
	}

	var fixedComponents, unfixedComponents []componentAndIndex
	for i, clc := range l.cs {
		if clc.fixed {
			fixedComponents = append(fixedComponents, componentAndIndex{clc.c, i})
		} else {
			unfixedComponents = append(unfixedComponents, componentAndIndex{clc.c, i})
		}
	}

	alloc := func(c Component, w, h int) (int, int, error) {
		reqWidth, reqHeight, err := c.Size(w, h)
		if err != nil {
			return 0, 0, err
		}
		reqLen, reqDepth := whToLd(reqWidth, reqHeight, l.dir)
		if reqLen == GROW {
			allocatedLen = GROW
		} else {
			allocatedLen += reqLen
		}
		if reqDepth > maxDepth {
			maxDepth = reqDepth
		}

		return reqWidth, reqHeight, nil
	}

	widths = make([]int, len(l.cs))
	heights = make([]int, len(l.cs))

	for _, c := range fixedComponents {
		len, dep := whToLd(width, height, l.dir)
		len -= allocatedLen
		widthRemainder, heightRemainder := ldToWh(len, dep, l.dir)
		w, h, err := alloc(c.c, widthRemainder, heightRemainder)
		if err != nil {
			return nil, nil, 0, 0, err
		}
		widths[c.index], heights[c.index] = w, h
	}

	if len(unfixedComponents) > 0 {
		lenPerUnfixed := (length - allocatedLen) / len(unfixedComponents)
		for _, c := range unfixedComponents {
			w, h := ldToWh(lenPerUnfixed, depth, l.dir)
			reqW, reqH, err := alloc(c.c, w, h)
			if err != nil {
				return nil, nil, 0, 0, err
			}
			widths[c.index], heights[c.index] = reqW, reqH
		}
	}

	return widths, heights, allocatedLen, maxDepth, nil
}

func (l *ConcatLayout) Size(width, height int) (int, int, error) {
	_, _, allocatedLen, maxDepth, err := l.allocate(width, height)
	if err != nil {
		return 0, 0, err
	}
	len, dep := ldToWh(allocatedLen, maxDepth, l.dir)
	return len, dep, err
}

func (l *ConcatLayout) Render(w Writer, width int, height int) error {
	widths, heights, _, _, err := l.allocate(width, height)
	if err != nil {
		return err
	}

	offset := 0
	for i, c := range l.cs {
		reqWidth, reqHeight, err := c.c.Size(widths[i], heights[i])
		if err != nil {
			return err
		}

		var subW Writer
		if l.dir == DirHor {
			var err error
			subW, err = w.Divide(offset, 0, reqWidth, reqHeight)
			if err != nil {
				return err
			}
			offset += reqWidth
		} else {
			var err error
			subW, err = w.Divide(0, offset, reqWidth, reqHeight)
			if err != nil {
				return err
			}
			offset += reqHeight
		}

		subW.RenderChild(c.c)
	}
	return nil
}

func NewLines() *ConcatLayout {
	return NewConcatLayout(DirVert)
}

type Line struct {
	del *FlexLayout
}

var _ Component = &Line{}

func NewLine() *Line {
	return &Line{del: NewFlexLayout(DirHor)}
}

func OneLine(c Component) *Line {
	l := NewLine()
	l.Add(c)
	return l
}

func (l *Line) Add(c Component) {
	l.del.Add(c)
}

func (l *Line) Size(width int, height int) (int, int, error) {
	return width, 1, nil
}

func (l *Line) Render(w Writer, width int, height int) error {
	w.SetContent(0, 0, 0, nil) // set at least one to take up our line
	w, err := w.Divide(0, 0, width, height)
	if err != nil {
		return err
	}
	w.RenderChild(l.del)
	return nil
}

// Fills a space by repeating a string
type FillerString struct {
	ch rune
}

var _ Component = &FillerString{}

func NewFillerString(ch rune) *FillerString {
	return &FillerString{ch: ch}
}

func (f *FillerString) Size(width int, height int) (int, int, error) {
	return width, 1, nil
}

func (f *FillerString) Render(w Writer, width int, height int) error {
	for i := 0; i < width; i++ {
		w.SetContent(i, 0, f.ch, nil)
	}
	return nil
}

type ColorLayout struct {
	del        Component
	color      tcell.Color
	foreground bool
}

var _ Component = &ColorLayout{}

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

func (l *ColorLayout) Size(width int, height int) (int, int, error) {
	return l.del.Size(width, height)
}

func (l *ColorLayout) Render(w Writer, width int, height int) error {
	if l.foreground {
		w = w.Foreground(l.color)
	} else {
		w = w.Background(l.color)
	}
	w, err := w.Fill()
	if err != nil {
		return err
	}
	w.RenderChild(l.del)
	return nil
}

type Box struct {
	focused bool
	title   string
	inner   Component
	grow    bool
}

var _ Component = &Box{}

// makes a box that will grow to fill its canvas
func NewGrowingBox() *Box {
	return &Box{grow: true}
}

// makes a new box that tightly wraps its inner component
func NewBox(inner Component) *Box {
	return &Box{inner: inner}
}

func (b *Box) SetInner(c Component) {
	b.inner = c
}

func (b *Box) SetFocused(focused bool) {
	b.focused = focused
}

func (b *Box) SetTitle(title string) {
	b.title = title
}

func (b *Box) Size(width int, height int) (int, int, error) {
	if b.grow {
		return width, height, nil
	} else {
		// +/-2 to account for the box chars themselves
		w, h, err := b.inner.Size(width-2, height-2)
		if err != nil {
			return 0, 0, err
		}
		return w + 2, h + 2, nil
	}
}

func (b *Box) Render(w Writer, width int, height int) error {
	if height == GROW && b.inner == nil {
		return fmt.Errorf("box must have either fixed height or a child")
	}

	width, height, err := b.Size(width, height)
	if err != nil {
		return err
	}

	if b.inner != nil {
		innerHeight := height - 2
		if height == GROW {
			innerHeight = GROW
		}

		w, err := w.Divide(1, 1, width-2, innerHeight)
		if err != nil {
			return err
		}

		childHeight := w.RenderChild(b.inner)
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

	if len(b.title) > 0 {
		middle := width / 2
		titleMargin := 3
		maxLength := width - (titleMargin * 2)
		renderedTitle := b.title
		if maxLength <= 0 {
			renderedTitle = ""
		} else if len(b.title) > maxLength {
			renderedTitle = renderedTitle[0:maxLength]
		}

		// don't add spaces if we can't fit any of the actual title in
		if len(renderedTitle) > 0 {
			renderedTitle = fmt.Sprintf(" %s ", renderedTitle)
		}

		start := middle - len(renderedTitle)/2
		for i, c := range renderedTitle {
			w.SetContent(start+i, 0, c, nil)
		}
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

var _ Component = &FixedSizeLayout{}

func NewFixedSize(del Component, width int, height int) *FixedSizeLayout {
	return &FixedSizeLayout{del: del, width: width, height: height}
}

func (l *FixedSizeLayout) Size(width int, height int) (int, int, error) {
	if l.width != GROW && l.height != GROW {
		return l.width, l.height, nil
	}
	rWidth, rHeight := l.width, l.height
	delWidth, delHeight, err := l.del.Size(width, height)
	if err != nil {
		return 0, 0, err
	}
	if rWidth == GROW {
		rWidth = delWidth
	}
	if rHeight == GROW {
		rHeight = delHeight
	}

	return rWidth, rHeight, nil
}

func (l *FixedSizeLayout) Render(w Writer, width int, height int) error {
	w.RenderChild(l.del)
	return nil
}

type ModalLayout struct {
	bg       Component
	fg       Component
	fraction float64
	fixed    bool
}

var _ Component = &ModalLayout{}

// fg will be rendered on top of bg
// if fixed is true, it will use using fraction/1 of the height and width of the screen
// if fixed is false, it will use whatever `fg` asks for, up to fraction/1 of width and height
func NewModalLayout(bg Component, fg Component, fraction float64, fixed bool) *ModalLayout {
	return &ModalLayout{fg: fg, bg: bg, fraction: fraction, fixed: fixed}
}

func (l *ModalLayout) Size(width int, height int) (int, int, error) {
	w, h, err := l.bg.Size(width, height)
	if err != nil {
		return 0, 0, err
	}
	if l.fraction > 1 {
		w = int(l.fraction * float64(w))
		h = int(l.fraction * float64(h))
	}

	return w, h, nil
}

func (l *ModalLayout) Render(w Writer, width int, height int) error {
	w.RenderChild(l.bg)

	var mw, mh int
	if !l.fixed {
		var err error
		mw, mh, err = l.fg.Size(int(l.fraction*float64(width)), int(l.fraction*float64(height)))
		if err != nil {
			return err
		}
	} else {
		f := (1 - l.fraction) / 2
		mw = int((1 - 2*f) * float64(width))
		mh = int((1 - 2*f) * float64(height))
	}

	mx := width/2 - mw/2
	my := height/2 - mh/2
	w, err := w.Divide(mx, my, mw, mh)
	if err != nil {
		return err
	}
	w.RenderChild(l.fg)
	return nil
}

type MinLengthLayout struct {
	inner     *ConcatLayout
	minLength int
	align     Align
}

func NewMinLengthLayout(len int, dir Dir) *MinLengthLayout {
	return &MinLengthLayout{
		inner:     NewConcatLayout(dir),
		minLength: len,
	}
}

func (l *MinLengthLayout) SetAlign(val Align) *MinLengthLayout {
	l.align = val
	return l
}

func (l *MinLengthLayout) Add(c Component) *MinLengthLayout {
	l.inner.Add(c)
	return l
}

func (ml *MinLengthLayout) Size(width int, height int) (int, int, error) {
	w, h, err := ml.inner.Size(width, height)
	if err != nil {
		return 0, 0, err
	}

	l, d := whToLd(w, h, ml.inner.dir)
	if l < ml.minLength {
		l = ml.minLength
	}
	w, h = ldToWh(l, d, ml.inner.dir)
	return w, h, err
}

func (ml *MinLengthLayout) Render(writer Writer, width int, height int) error {
	if ml.align == AlignEnd {
		w, h, err := ml.inner.Size(width, height)
		if err != nil {
			return err
		}

		indentW := 0
		indentH := 0
		if ml.inner.dir == DirHor {
			indentW = width - w
		} else {
			indentH = height - h
		}
		if indentW != 0 || indentH != 0 {
			subW, err := writer.Divide(indentW, indentH, w, h)
			if err != nil {
				return err
			}
			return ml.inner.Render(subW, w, h)
		}
	}

	return ml.inner.Render(writer, width, height)
}
