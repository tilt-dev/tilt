package rty

type ScrollComponent interface {
	ID() ID
	Render(w Writer, prevState interface{}, width, height int) (state interface{}, err error)
}

func newLineProvenanceWriter() *lineProvenanceWriter {
	return &lineProvenanceWriter{}
}

type lineProvenanceWriter struct {
	del    []FQID
	offset int
}

func (p *lineProvenanceWriter) WriteLineProvenance(fqid FQID, start int, end int) {
	if p == nil {
		return
	}
	for len(p.del) <= end+p.offset {
		p.del = append(p.del, "")
	}
	for i := start; i <= end; i++ { // use <= instead of < to record one-line provenances
		if len(fqid) > len(p.del[p.offset+i]) {
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
