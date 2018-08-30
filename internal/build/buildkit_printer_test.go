package build

import (
	"strings"
	"testing"
)

func TestBuildkitPrinter(t *testing.T) {
	output := &strings.Builder{}
	p := newBuildkitPrinter(output)

	p.parseAndPrint(vertexes, logs)

	expected := `RUN: (>&2 echo "error"; exit 1)
	→ ERROR: error`

	if output.String() != expected {
		t.Errorf("Expected %s, got %s", expected, output.String())
	}
}
