package build

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	controlapi "github.com/moby/buildkit/api/services/control"

	"github.com/windmilleng/tilt/internal/logger"
)

// NOTE(dmiller): set at runtime with:
// go test -ldflags="-X github.com/windmilleng/tilt/internal/build.WriteGoldenMaster=1" github.com/windmilleng/tilt/internal/build -run ^TestBuildkitPrinter
var WriteGoldenMaster = "0"

type buildkitTestCase struct {
	name         string
	level        logger.Level
	responsePath string
}

func (c buildkitTestCase) readResponse(reader io.Reader) ([]controlapi.StatusResponse, error) {
	result := make([]controlapi.StatusResponse, 0)
	decoder := json.NewDecoder(reader)
	for decoder.More() {
		var resp controlapi.StatusResponse
		err := decoder.Decode(&resp)
		if err != nil {
			return nil, err
		}
		result = append(result, resp)
	}
	return result, nil
}

func TestBuildkitPrinter(t *testing.T) {
	cases := []buildkitTestCase{
		{"echo-hi-success", logger.InfoLvl, "echo-hi-success.response.txt"},
		{"echo-hi-success-verbose", logger.VerboseLvl, "echo-hi-success.response.txt"},
		{"echo-hi-failure", logger.InfoLvl, "echo-hi-failure.response.txt"},
		{"echo-hi-failure-verbose", logger.InfoLvl, "echo-hi-failure.response.txt"},
		{"multistage-success", logger.InfoLvl, "multistage-success.response.txt"},
		{"multistage-fail-run", logger.InfoLvl, "multistage-fail-run.response.txt"},
		{"multistage-fail-copy", logger.InfoLvl, "multistage-fail-copy.response.txt"},
	}

	base := t.Name()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fullPath := fmt.Sprintf("testdata/%s/%s", base, c.responsePath)
			f, err := os.Open(fullPath)
			if err != nil {
				t.Fatal(err)
			}

			responses, err := c.readResponse(f)
			if err != nil {
				t.Fatal(err)
			}

			output := &strings.Builder{}
			logger := logger.NewLogger(c.level, output)
			p := newBuildkitPrinter(logger)

			for _, resp := range responses {
				err := p.parseAndPrint(toVertexes(resp))
				if err != nil {
					t.Fatal(err)
				}
			}

			d1 := []byte(output.String())
			gmPath := fmt.Sprintf("testdata/%s.master.txt", t.Name())
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
