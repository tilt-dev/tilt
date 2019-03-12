package build

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"

	"github.com/windmilleng/tilt/internal/logger"
)

// NOTE(dmiller): set at runtime with:
// go test -ldflags="-X github.com/windmilleng/tilt/internal/build.WriteGoldenMaster=1" github.com/windmilleng/tilt/internal/build -run ^TestBuildkitPrinter
var WriteGoldenMaster = "0"

type buildkitTestCase struct {
	name     string
	level    logger.Level
	vertices []*vertex
	logs     []*vertexLog
}

var digests = []digest.Digest{
	"sha8234234546454",
	"sha1234234234234",
	"sha82342xxxx454",
}

var shCmdVertexNames = []string{
	"/bin/sh -c make",
	`/bin/sh -c (>&2 echo "hi")`,
	"docker-image://docker.io/blah",
}

var stepVertexNames = []string{
	"[1/2] RUN make",
	`[2/2] RUN echo "hi"`,
	"docker-image://docker.io/blah",
}

func buildkitTestCase1() buildkitTestCase {
	return buildkitTestCase{
		name:  "echo-hi-error",
		level: logger.InfoLvl,
		vertices: []*vertex{
			{
				digest: digests[0],
				name:   shCmdVertexNames[0],
				error:  "",
			},
			{
				digest:  digests[0],
				name:    shCmdVertexNames[0],
				error:   "",
				started: true,
			},
			{
				digest:    digests[0],
				name:      shCmdVertexNames[0],
				error:     "",
				started:   true,
				completed: true,
			},
			{
				digest: digests[1],
				name:   shCmdVertexNames[1],
				error:  "",
			},
			{
				digest:  digests[1],
				name:    shCmdVertexNames[1],
				error:   "",
				started: true,
			},
			{
				digest:    digests[1],
				name:      shCmdVertexNames[1],
				error:     "context canceled",
				started:   true,
				completed: true,
			},
			{
				digest: digests[2],
				name:   shCmdVertexNames[2],
				error:  "",
			},
			{
				digest:  digests[2],
				name:    shCmdVertexNames[2],
				error:   "",
				started: true,
			},
			{
				digest:    digests[1],
				name:      shCmdVertexNames[1],
				error:     "",
				started:   true,
				completed: true,
			},
		},
		logs: []*vertexLog{
			{
				vertex: digests[1],
				msg:    []byte("hi"),
			},
			{
				vertex: digests[0],
				msg:    []byte(""),
			},
		},
	}
}

func buildkitTestCase2() buildkitTestCase {
	return buildkitTestCase{
		name:  "echo-hi-success",
		level: logger.InfoLvl,
		vertices: []*vertex{
			{
				digest: digests[0],
				name:   shCmdVertexNames[0],
				error:  "",
			},
			{
				digest:  digests[0],
				name:    shCmdVertexNames[0],
				error:   "",
				started: true,
			},
			{
				digest:    digests[0],
				name:      shCmdVertexNames[0],
				error:     "",
				started:   true,
				completed: true,
			},
			{
				digest: digests[1],
				name:   shCmdVertexNames[1],
				error:  "",
			},
			{
				digest:  digests[1],
				name:    shCmdVertexNames[1],
				error:   "",
				started: true,
			},
			{
				digest:    digests[1],
				name:      shCmdVertexNames[1],
				error:     "",
				started:   true,
				completed: true,
			},
			{
				digest: digests[2],
				name:   shCmdVertexNames[2],
				error:  "",
			},
			{
				digest:  digests[2],
				name:    shCmdVertexNames[2],
				error:   "",
				started: true,
			},
			{
				digest:    digests[1],
				name:      shCmdVertexNames[1],
				error:     "",
				started:   true,
				completed: true,
			},
		},
		logs: []*vertexLog{
			{
				vertex: digests[1],
				msg:    []byte("hi"),
			},
			{
				vertex: digests[0],
				msg:    []byte(""),
			},
		},
	}
}

func buildkitTestCase3() buildkitTestCase {
	c := buildkitTestCase2()
	c.name = "echo-hi-success-verbose"
	c.level = logger.VerboseLvl
	return c
}

func buildkitTestCase4() buildkitTestCase {
	return buildkitTestCase{
		name:  "docker-18.09-output",
		level: logger.InfoLvl,
		vertices: []*vertex{
			{
				digest: digests[0],
				name:   stepVertexNames[0],
				error:  "",
			},
			{
				digest:  digests[0],
				name:    stepVertexNames[0],
				error:   "",
				started: true,
			},
			{
				digest:    digests[0],
				name:      stepVertexNames[0],
				error:     "",
				started:   true,
				completed: true,
			},
			{
				digest: digests[1],
				name:   stepVertexNames[1],
				error:  "",
			},
			{
				digest:  digests[1],
				name:    stepVertexNames[1],
				error:   "",
				started: true,
			},
			{
				digest:    digests[1],
				name:      stepVertexNames[1],
				error:     "context canceled",
				started:   true,
				completed: true,
			},
			{
				digest: digests[2],
				name:   stepVertexNames[2],
				error:  "",
			},
			{
				digest:  digests[2],
				name:    stepVertexNames[2],
				error:   "",
				started: true,
			},
			{
				digest:    digests[1],
				name:      stepVertexNames[1],
				error:     "",
				started:   true,
				completed: true,
			},
		},
		logs: []*vertexLog{
			{
				vertex: digests[1],
				msg:    []byte("hi"),
			},
			{
				vertex: digests[0],
				msg:    []byte("hello"),
			},
		},
	}
}

func TestBuildkitPrinter(t *testing.T) {
	cases := []buildkitTestCase{
		buildkitTestCase1(),
		buildkitTestCase2(),
		buildkitTestCase3(),
		buildkitTestCase4(),
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			output := &strings.Builder{}
			logger := logger.NewLogger(c.level, output)
			p := newBuildkitPrinter(logger)
			err := p.parseAndPrint(c.vertices, c.logs)
			if err != nil {
				t.Fatal(err)
			}

			d1 := []byte(output.String())
			gmPath := fmt.Sprintf("testdata/%s_master", t.Name())
			if WriteGoldenMaster == "1" {
				err := os.MkdirAll(filepath.Dir(gmPath), 0777)
				if err != nil {
					t.Fatal(err)
				}

				err = ioutil.WriteFile(gmPath, d1, 0644)
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
		})
	}
}
