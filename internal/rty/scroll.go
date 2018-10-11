package rty

import (
	"fmt"
	"strings"
)

type ScrollComponent interface {
	ID() ID
	RenderScroll(w Writer, prevState interface{}, width, height int) (state interface{}, err error)
}

func newLineProvenanceWriter() *lineProvenanceWriter {
	return &lineProvenanceWriter{}
}

type lineProvenanceWriter struct {
	del    []FQID
	offset int
}

func (p *lineProvenanceWriter) WriteLineProvenance(fqid FQID, numLines int) {
	if p == nil {
		return
	}
	for len(p.del) < p.offset+numLines {
		p.del = append(p.del, "")
	}
	for i := 0; i < numLines; i++ {
		current := p.del[p.offset+i]
		if len(fqid) > len(current) {
			p.del[p.offset+i] = fqid
		}
	}
}

func (p *lineProvenanceWriter) Divide(offset int) *lineProvenanceWriter {
	if p == nil {
		return (*lineProvenanceWriter)(nil)
	}

	return &lineProvenanceWriter{del: p.del, offset: p.offset + offset}
}

func (p *lineProvenanceWriter) data() *LineProvenanceData {
	if p == nil {
		return nil
	}
	return &LineProvenanceData{append([]FQID(nil), p.del...)}
}

type WrapScrollComponent struct {
	del ScrollComponent
}

func NewWrapScrollComponent(del ScrollComponent) *WrapScrollComponent {
	return &WrapScrollComponent{del: del}
}

func (l *WrapScrollComponent) ID() ID {
	return ""
}

func (l *WrapScrollComponent) Size(width, height int) (int, int) {
	return width, height
}

func (l *WrapScrollComponent) Render(w Writer, width, height int) error {
	w.RenderChildScroll(l.del)
	return nil
}

type TextScrollLayout struct {
	id ID
	cs []Component
}

func NewTextScrollLayout(id ID) *TextScrollLayout {
	return &TextScrollLayout{
		id: id,
	}
}

func (l *TextScrollLayout) Add(c Component) {
	l.cs = append(l.cs, c)
}

func (l *TextScrollLayout) ID() ID {
	return l.id
}

func (l *TextScrollLayout) Size(width int, height int) (int, int) {
	return width, height
}

func (l *TextScrollLayout) RenderScroll(w Writer, prevState interface{}, width, height int) (state interface{}, err error) {
	prev, ok := prevState.(*TextScrollState)
	if !ok {
		prev = &TextScrollState{idx: 5}
	}
	next := &TextScrollState{
		width:  width,
		height: height,
	}

	if len(l.cs) == 0 {
		return next, nil
	}

	var canvases []Canvas

	for _, c := range l.cs {
		childCanvas, childProvenance := w.RenderChildInTemp(c)
		canvases = append(canvases, childCanvas)
		next.provenance = append(next.provenance, childProvenance.Data...)
	}

	skip := l.findInitialIdx(prev, next)
	canvasIdx := 0
	srcY := 0
	for skip > 0 {
		canvas := canvases[canvasIdx]
		_, canvasHeight := canvas.Size()
		if canvasHeight <= skip {
			skip -= canvasHeight
			canvasIdx++
		} else {
			srcY = skip
			skip = 0
		}
	}

	y := 0
	remaining := height
	for remaining > 0 && canvasIdx < len(canvases) {
		canvas := canvases[canvasIdx]
		_, canvasHeight := canvas.Size()
		numLines := canvasHeight - srcY
		if numLines > remaining {
			numLines = remaining
		}
		w.Divide(0, y, width, numLines).Embed(canvas, srcY, numLines)
		y += numLines
		remaining -= numLines
		srcY = 0
		canvasIdx++
	}
	return next, nil
}

func (l *TextScrollLayout) findInitialIdx(prev *TextScrollState, next *TextScrollState) int {
	if len(prev.provenance) == 0 {
		return 0
	}

	prevTopFqid := prev.provenance[prev.idx]
	prevLineInElem := 0

	for _, fqid := range prev.provenance[0:prev.idx] {
		if fqid == prevTopFqid {
			prevLineInElem++
		}
	}

	bestIdx := 0
	lineInElem := -1
	for i, fqid := range next.provenance {
		if fqid == prevTopFqid {
			bestIdx = i
			lineInElem++
			if lineInElem == prevLineInElem {
				break
			}
		}
	}

	next.idx = bestIdx
	return bestIdx
}

type TextScrollState struct {
	width  int
	height int

	idx        int
	provenance []FQID
}

type TextScrollController struct {
	state *TextScrollState
}

func NewTextScrollController(state *TextScrollState) *TextScrollController {
	return &TextScrollController{state: state}
}

func (s *TextScrollController) Up() {
	if s.state.idx == 0 {
		return
	}

	s.state.idx = s.state.idx - 1
}

func (s *TextScrollController) Down() {
	if s.state.idx == len(s.state.provenance)-1 {
		return
	}

	s.state.idx = s.state.idx + 1
}

func NewScrollingWrappingTextArea(id ID, text string) Component {
	l := NewTextScrollLayout(id)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		l.Add(NewWrappingTextLine(ID(fmt.Sprintf("line-%06d", i)), line))
	}

	return NewWrapScrollComponent(l)
}

// type ScrollLayout struct {
// 	concat *ConcatLayout
// }

// func NewScrollLayout(dir Dir) *ScrollLayout {
// 	if dir == DirHor {
// 		panic(fmt.Errorf("ScrollLayout doesn't support Horizontal"))
// 	}

// 	return &ScrollLayout{
// 		concat: NewConcatLayout(DirVert),
// 	}
// }

// func (l *ScrollLayout) Add(c FixedDimComponent) {
// 	l.concat.Add(c)
// }

// func (l *ScrollLayout) Render(c CanvasWriter) error {
// 	width, height := c.Size()
// 	if height < l.concat.FixedDimSize() {
// 		height = l.concat.FixedDimSize()
// 	}
// 	tempC := NewTempCanvas(width, height)
// 	if err := l.concat.Render(tempC); err != nil {
// 		return err
// 	}

// 	Copy(tempC, c)
// 	return nil
// }
