package build

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
)

const writeGoldenMaster = false

func TestBuildkitPrinter(t *testing.T) {
	output := &strings.Builder{}
	p := newBuildkitPrinter(output)

	vertexes := []*vertex{
		{
			digest: "sha8234234546454",
			name:   "/bin/sh -c make",
			error:  "",
		},
		{
			digest:  "sha8234234546454",
			name:    "/bin/sh -c make",
			error:   "",
			started: true,
		},
		{
			digest:    "sha8234234546454",
			name:      "/bin/sh -c make",
			error:     "",
			started:   true,
			completed: true,
		},
		{
			digest: "sha1234234234234",
			name:   `/bin/sh -c (>&2 echo "hi")`,
			error:  "",
		},
		{
			digest:  "sha1234234234234",
			name:    `/bin/sh -c (>&2 echo "hi")`,
			error:   "",
			started: true,
		},
		{
			digest:    "sha1234234234234",
			name:      `/bin/sh -c (>&2 echo "hi")`,
			error:     "context canceled",
			started:   true,
			completed: true,
		},
		{
			digest: "sha82342xxxx454",
			name:   "docker-image://docker.io/blah",
			error:  "",
		},
		{
			digest:  "sha82342xxxx454",
			name:    "docker-image://docker.io/blah",
			error:   "",
			started: true,
		},
		{
			digest:    "sha1234234234234",
			name:      `/bin/sh -c (>&2 echo "hi")`,
			error:     "",
			started:   true,
			completed: true,
		},
	}
	logs := []*vertexLog{
		{
			vertex: "sha1234234234234",
			msg:    []byte("hi"),
		},
		{
			vertex: "sha8234234546454",
			msg:    []byte(""),
		},
	}

	err := p.parseAndPrint(vertexes, logs)
	if err != nil {
		t.Fatal(err)
	}

	d1 := []byte(output.String())
	gmPath := fmt.Sprintf("testdata/%s_master", t.Name())
	if writeGoldenMaster {
		err := ioutil.WriteFile(gmPath, d1, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	expected, err := ioutil.ReadFile(gmPath)
	if err != nil {
		t.Fatal(err)
	}

	if output.String() != string(expected) {
		t.Errorf("EXPECTED:\n%s\nGOT:\n%s\n", expected, output.String())
	}
}
