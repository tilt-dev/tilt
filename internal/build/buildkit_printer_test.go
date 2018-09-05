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
			error:  "",
		},
		{
			digest: "sha8234234546454",
			name:   "/bin/sh -c echo hi",
			error:  "",
		},
		{
			digest: "sha82342xxxx454",
			name:   "docker-image://docker.io/blah",
			error:  "",
		},
		{
			digest: "sha1234234234234",
			name:   `/bin/sh -c (>&2 echo "error"; exit 1)`,
			error:  "",
		},
	}
	logs := []*vertexLog{
		{
			vertex: "sha1234234234234",
			msg:    []byte("error"),
		},
		{
			vertex: "sha8234234546454",
			msg:    []byte(""),
		},
	}

	p.parse(vertexes, logs)

	expected := `    ╎ RUN: (>&2 echo "error"; exit 1)
    ╎   → ERROR: error
    ╎ RUN: echo hi
`

	err := p.print()
	if err != nil {
		t.Fatal(err)
	}

	if output.String() != expected {
		t.Errorf("EXPECTED:\n%s\nGOT:\n%s\n", expected, output.String())
	}
}
