package rty

import (
	"testing"

	"github.com/gdamore/tcell"
	"github.com/stretchr/testify/assert"
)

func BenchmarkNestedFlexLayouts(b *testing.B) {

	sc := tcell.NewSimulationScreen("")
	err := sc.Init()
	assert.NoError(b, err)
	sc.SetSize(100, 100)

	r := NewRTY(sc, b)

	run := func() {

		topF := NewFlexLayout(DirVert)
		innerF := topF
		for i := 0; i < 100; i++ {
			newF := NewFlexLayout(DirHor)
			innerF.Add(newF)
			innerF = newF
		}

		innerF.Add(TextString("hello"))
		r.Render(topF)
	}
	for i := 0; i < b.N; i++ {
		run()
	}
}
