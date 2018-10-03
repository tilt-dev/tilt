package hud

import "sync"

type Model struct {
	Mu sync.Mutex

	Info string
}

func NewModel() *Model {
	return &Model{
		Mu:   sync.Mutex{},
		Info: "empty HUD model",
	}
}
