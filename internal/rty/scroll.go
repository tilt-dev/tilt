package rty

import (
	"strings"
)

type StatefulComponent interface {
	RenderStateful(w Writer, prevState interface{}, width, height int) (state interface{}, err error)
}

type TextScrollLayout struct {
	name string
	cs   []Component
}

var _ Component = &TextScrollLayout{}

func NewTextScrollLayout(name string) *TextScrollLayout {
	return &TextScrollLayout{name: name}
}

func (l *TextScrollLayout) Add(c Component) {
	l.cs = append(l.cs, c)
}

func (l *TextScrollLayout) Size(width int, height int) (int, int, error) {
	return width, height, nil
}

type TextScrollState struct {
	width  int
	height int

	canvasIdx     int
	lineIdx       int // line within canvas
	canvasLengths []int

	following bool
}

func defaultTextScrollState() *TextScrollState {
	return &TextScrollState{following: true}
}
func (l *TextScrollLayout) Render(w Writer, width, height int) error {
	w.RenderStateful(l, l.name)
	return nil
}

func (l *TextScrollLayout) RenderStateful(w Writer, prevState interface{}, width, height int) (state interface{}, err error) {
	prev, ok := prevState.(*TextScrollState)
	if !ok {
		prev = defaultTextScrollState()
	}
	next := &TextScrollState{
		width:     width,
		height:    height,
		following: prev.following,
	}

	if len(l.cs) == 0 {
		return next, nil
	}

	scrollbarWriter, err := w.Divide(width-1, 0, 1, height)
	if err != nil {
		return nil, err
	}
	w, err = w.Divide(0, 0, width-1, height)
	if err != nil {
		return nil, err
	}

	next.canvasLengths = make([]int, len(l.cs))
	canvases := make([]Canvas, len(l.cs))

	for i, c := range l.cs {
		childCanvas := w.RenderChildInTemp(c)
		canvases[i] = childCanvas
		_, childHeight := childCanvas.Size()
		next.canvasLengths[i] = childHeight
	}

	l.adjustCursor(prev, next, canvases)

	y := 0
	canvases = canvases[next.canvasIdx:]

	if next.lineIdx != 0 {
		firstCanvas := canvases[0]
		canvases = canvases[1:]
		_, firstHeight := firstCanvas.Size()
		numLines := firstHeight - prev.lineIdx
		if numLines > height {
			numLines = height
		}

		w, err := w.Divide(0, 0, width-1, numLines)
		if err != nil {
			return nil, err
		}

		err = w.Embed(firstCanvas, next.lineIdx, numLines)
		if err != nil {
			return nil, err
		}
		y += numLines
	}

	for _, canvas := range canvases {
		_, canvasHeight := canvas.Size()
		numLines := canvasHeight
		if numLines > height-y {
			numLines = height - y
		}
		w, err := w.Divide(0, y, width-1, numLines)
		if err != nil {
			return nil, err
		}

		err = w.Embed(canvas, 0, numLines)
		if err != nil {
			return nil, err
		}
		y += numLines
	}

	if height >= 2 {
		if next.lineIdx > 0 || next.canvasIdx > 0 {
			scrollbarWriter.SetContent(0, 0, '↑', nil)
		}

		if y >= height && !next.following {
			scrollbarWriter.SetContent(0, height-1, '↓', nil)
		}
	}

	return next, nil
}

func (l *TextScrollLayout) adjustCursor(prev *TextScrollState, next *TextScrollState, canvases []Canvas) {
	if next.following {
		next.jumpToBottom(canvases)
		return
	}

	if prev.canvasIdx >= len(canvases) {
		return
	}

	next.canvasIdx = prev.canvasIdx
	_, canvasHeight := canvases[next.canvasIdx].Size()
	if prev.lineIdx >= canvasHeight {
		return
	}
	next.lineIdx = prev.lineIdx
}

func (s *TextScrollState) jumpToBottom(canvases []Canvas) {
	totalHeight := totalHeight(canvases)
	if totalHeight <= s.height {
		// all content fits on the screen
		s.canvasIdx = 0
		s.lineIdx = 0
		return
	}

	heightLeft := s.height
	for i := range canvases {
		// we actually want to iterate from the end
		iEnd := len(canvases) - i - 1
		c := canvases[iEnd]

		_, cHeight := c.Size()
		if cHeight < heightLeft {
			heightLeft -= cHeight
		} else if cHeight == heightLeft {
			// start at the beginning of this canvas
			s.canvasIdx = iEnd
			s.lineIdx = 0
			return
		} else {
			// start some number of lines into this canvas.
			s.canvasIdx = iEnd
			s.lineIdx = cHeight - heightLeft
			return
		}
	}
}

type TextScrollController struct {
	state *TextScrollState
}

func (s *TextScrollController) Top() {
	st := s.state
	if st.canvasIdx != 0 || st.lineIdx != 0 {
		s.SetFollow(false)
	}
	st.canvasIdx = 0
	st.lineIdx = 0
}

func (s *TextScrollController) Bottom() {
	s.SetFollow(true)
}

func (s *TextScrollController) Up() {
	st := s.state
	if st.lineIdx != 0 {
		s.SetFollow(false)
		st.lineIdx--
		return
	}

	if st.canvasIdx == 0 {
		return
	}
	s.SetFollow(false)
	st.canvasIdx--
	st.lineIdx = st.canvasLengths[st.canvasIdx] - 1
}

func (s *TextScrollController) Down() {
	st := s.state

	if st.following {
		return
	}

	if len(st.canvasLengths) == 0 {
		return
	}

	canvasLength := st.canvasLengths[st.canvasIdx]
	if st.lineIdx+st.height < canvasLength-1 {
		// we can just go down in this canvas
		st.lineIdx++
		return
	}
	if st.canvasIdx == len(st.canvasLengths)-1 {
		// we're at the end of the last canvas
		s.SetFollow(true)
		return
	}
	st.canvasIdx++
	st.lineIdx = 0
}

func (s *TextScrollController) ToggleFollow() {
	s.state.following = !s.state.following
}

func (s *TextScrollController) SetFollow(follow bool) {
	s.state.following = follow
}

func NewScrollingWrappingTextArea(name string, text string) Component {
	l := NewTextScrollLayout(name)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		l.Add(TextString(line + "\n"))
	}
	return l
}

type ElementScrollLayout struct {
	name     string
	children []Component
}

var _ Component = &ElementScrollLayout{}

func NewElementScrollLayout(name string) *ElementScrollLayout {
	return &ElementScrollLayout{name: name}
}

func (l *ElementScrollLayout) Add(c Component) {
	l.children = append(l.children, c)
}

func (l *ElementScrollLayout) Size(width int, height int) (int, int, error) {
	return width, height, nil
}

type ElementScrollState struct {
	width  int
	height int

	firstVisibleElement int

	children []string

	elementIdx int
}

func (l *ElementScrollLayout) Render(w Writer, width, height int) error {
	w.RenderStateful(l, l.name)
	return nil
}

func (l *ElementScrollLayout) RenderStateful(w Writer, prevState interface{}, width, height int) (state interface{}, err error) {
	prev, ok := prevState.(*ElementScrollState)
	if !ok {
		prev = &ElementScrollState{}
	}

	next := *prev
	next.width = width
	next.height = height

	if len(l.children) == 0 {
		return &next, nil
	}

	scrollbarWriter, err := w.Divide(width-1, 0, 1, height)
	if err != nil {
		return nil, err
	}
	w, err = w.Divide(0, 0, width-1, height)
	if err != nil {
		return nil, err
	}

	var canvases []Canvas
	var heights []int
	for _, c := range l.children {
		canvas := w.RenderChildInTemp(c)
		canvases = append(canvases, canvas)
		_, childHeight := canvas.Size()
		heights = append(heights, childHeight)
	}

	next.firstVisibleElement = calculateFirstVisibleElement(next, heights, height)

	y := 0
	showDownArrow := false
	for i, h := range heights {
		if i >= next.firstVisibleElement {
			if h > height-y {
				h = height - y
				showDownArrow = true
			}
			w, err := w.Divide(0, y, width-1, h)
			if err != nil {
				return nil, err
			}

			err = w.Embed(canvases[i], 0, h)
			if err != nil {
				return nil, err
			}
			y += h
		}
	}

	if next.firstVisibleElement != 0 {
		scrollbarWriter.SetContent(0, 0, '↑', nil)
	}

	if showDownArrow {
		scrollbarWriter.SetContent(0, height-1, '↓', nil)
	}

	return &next, nil
}

func calculateFirstVisibleElement(state ElementScrollState, heights []int, height int) int {
	if state.elementIdx < state.firstVisibleElement {
		// if we've scrolled back above the old first visible element, just make the selected element the first visible
		return state.elementIdx
	} else if state.elementIdx > state.firstVisibleElement {
		var lastLineOfSelectedElement int
		for i := state.firstVisibleElement; i < state.elementIdx+1 && i < len(heights); i++ {
			lastLineOfSelectedElement += heights[i]
		}

		if lastLineOfSelectedElement > height {
			// the selected element isn't fully visible, so start from that element and work backwards, adding previous elements
			// as long as they'll fit on the screen
			if lastLineOfSelectedElement > state.height {
				firstVisibleElement := state.elementIdx
				heightUsed := heights[firstVisibleElement]
				for firstVisibleElement > 0 {
					prevHeight := heights[firstVisibleElement-1]
					if heightUsed+prevHeight > state.height {
						break
					}
					firstVisibleElement--
					heightUsed += prevHeight
				}
				return firstVisibleElement
			}
		}
	}

	return state.firstVisibleElement
}

type ElementScrollController struct {
	state *ElementScrollState
}

func adjustElementScroll(prevInt interface{}, newChildren []string) (*ElementScrollState, string) {
	prev, ok := prevInt.(*ElementScrollState)
	if !ok {
		prev = &ElementScrollState{}
	}

	clone := *prev
	next := &clone
	next.children = newChildren

	if len(newChildren) == 0 {
		next.elementIdx = 0
		return next, ""
	}
	if len(prev.children) == 0 {
		sel := ""
		if len(next.children) > 0 {
			sel = next.children[0]
		}
		return next, sel
	}
	if prev.elementIdx >= len(prev.children) {
		// NB(dbentley): this should be impossible, but we were hitting it and it was crashing
		next.elementIdx = 0
		return next, ""
	}
	prevChild := prev.children[prev.elementIdx]
	for i, child := range newChildren {
		if child == prevChild {
			next.elementIdx = i
			return next, child
		}
	}
	return next, next.children[0]
}

func (s *ElementScrollController) GetSelectedIndex() int {
	return s.state.elementIdx
}

func (s *ElementScrollController) GetSelectedChild() string {
	if len(s.state.children) == 0 {
		return ""
	}
	return s.state.children[s.state.elementIdx]
}

func (s *ElementScrollController) Up() {
	if s.state.elementIdx == 0 {
		return
	}

	s.state.elementIdx--
}

func (s *ElementScrollController) Down() {
	if s.state.elementIdx == len(s.state.children)-1 {
		return
	}
	s.state.elementIdx++
}

func (s *ElementScrollController) Top() {
	s.state.elementIdx = 0
}

func (s *ElementScrollController) Bottom() {
	s.state.elementIdx = len(s.state.children) - 1
}
