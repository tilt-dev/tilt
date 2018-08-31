package build

import (
	"strings"
	"testing"
)

func TestBuildkitPrinter(t *testing.T) {
	output := &strings.Builder{}
	p := newBuildkitPrinter(output)

	vertexes := []*vertex{
		{
			digest: "sha1234234234234",
			name:   `/bin/sh -c (>&2 echo "error"; exit 1)`,
		},
	}
	logs := []*vertexLog{
		{
			vertex: "sha1234234234234",
			msg:    []byte("error"),
		},
	}

	p.parseAndPrint(vertexes, logs)

	expected := `RUN: (>&2 echo "error"; exit 1)
→ ERROR: error`

	if output.String() != expected {
		t.Errorf("Expected %s. Got %s", expected, output.String())
	}
}
