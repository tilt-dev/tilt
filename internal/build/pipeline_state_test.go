package build

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/windmilleng/tilt/internal/logger"
)

// NOTE(dmiller): set at runtime with:
// go test -ldflags="-X 'github.com/windmilleng/tilt/internal/build.PipelineStateWriteGoldenMaster=1'" github.com/windmilleng/tilt/internal/build -run ^TestPipeline
var PipelineStateWriteGoldenMaster = "0"

func TestPipeline(t *testing.T) {
	var err error
	out := &bytes.Buffer{}
	ctx := logger.WithLogger(context.Background(), logger.NewLogger(logger.InfoLvl, out))
	ps := NewPipelineState(ctx, 1, fakeClock{})
	ps.StartPipelineStep(ctx, "%s %s", "hello", "world")
	ps.Printf(ctx, "in ur step")
	ps.EndPipelineStep(ctx)
	ps.End(ctx, err)

	assertSnapshot(t, out.String())
}

func TestPipelineErrored(t *testing.T) {
	err := fmt.Errorf("oh noes")
	out := &bytes.Buffer{}
	ctx := logger.WithLogger(context.Background(), logger.NewLogger(logger.InfoLvl, out))
	ps := NewPipelineState(ctx, 1, fakeClock{})
	ps.StartPipelineStep(ctx, "%s %s", "hello", "world")
	ps.Printf(ctx, "in ur step")
	ps.EndPipelineStep(ctx)
	ps.End(ctx, err)

	assertSnapshot(t, out.String())
}

func TestPipelineMultilinePrint(t *testing.T) {
	var err error
	out := &bytes.Buffer{}
	ctx := logger.WithLogger(context.Background(), logger.NewLogger(logger.InfoLvl, out))
	ps := NewPipelineState(ctx, 1, fakeClock{})
	ps.StartPipelineStep(ctx, "%s %s", "hello", "world")
	ps.Printf(ctx, "line 1\nline 2\n")
	ps.EndPipelineStep(ctx)
	ps.End(ctx, err)

	assertSnapshot(t, out.String())
}

func assertSnapshot(t *testing.T, output string) {
	d1 := []byte(output)
	gmPath := fmt.Sprintf("testdata/%s_master", t.Name())
	if PipelineStateWriteGoldenMaster == "1" {
		err := ioutil.WriteFile(gmPath, d1, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	expected, err := ioutil.ReadFile(gmPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(expected) != output {
		t.Errorf("Expected: %s != Output: %s", expected, output)
	}
}
