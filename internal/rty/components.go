package rty

// Components are able to render themselves onto a screen

// Component renders onto a canvas
type Component interface {
	Render(c CanvasWriter) error
}

type Dir int

const (
	DirHor Dir = iota
	DirVert
)
