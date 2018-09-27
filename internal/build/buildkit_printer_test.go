package build

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func buildkitTestCase1() buildkitTestCase {
	return buildkitTestCase{
		name:  "echo-hi-error",
		level: logger.InfoLvl,
		vertices: []*vertex{
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
		},
		logs: []*vertexLog{
			{
				vertex: "sha1234234234234",
				msg:    []byte("hi"),
			},
			{
				vertex: "sha8234234546454",
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
				error:     "",
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
		},
		logs: []*vertexLog{
			{
				vertex: "sha1234234234234",
				msg:    []byte("hi"),
			},
			{
				vertex: "sha8234234546454",
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

func TestBuildkitPrinter(t *testing.T) {
	cases := []buildkitTestCase{
		buildkitTestCase1(),
		buildkitTestCase2(),
		buildkitTestCase3(),
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
