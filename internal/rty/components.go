package rty

// Components are able to render themselves onto a screen

type ComponentID string

type FQID string

// Component renders onto a canvas
type Component interface {
	ID() ComponentID
	Size(availWidth, availHeight int) (int, int)
	Render(w Writer, width, height int) error
}

type Dir int

const (
	DirHor Dir = iota
	DirVert
)

// XXX(dbentley): delete
// FixedDimComponent has a fixed size in one dimension (for use in a FlexLayout)
type FixedDimComponent interface {
	Component
	FixedDimSize() int
}
