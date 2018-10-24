package rty

import (
	"testing"

	"github.com/windmilleng/tcell"
)

var screen tcell.Screen

func TestMain(m *testing.M) {
	InitScreenAndRun(m, &screen)
}
