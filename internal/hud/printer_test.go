package hud

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/pkg/model/logstore"
)

func TestPrinterProgressBackoff(t *testing.T) {
	out := &bytes.Buffer{}
	now := time.Now()
	printer := NewIncrementalPrinter(Stdout(out))

	printer.Print([]logstore.LogLine{
		logstore.LogLine{Text: "layer 1: Pending\n", ProgressID: "layer 1", Time: now},
		logstore.LogLine{Text: "layer 2: Pending\n", ProgressID: "layer 2", Time: now},
	})

	assert.Equal(t, "layer 1: Pending\nlayer 2: Pending\n", out.String())

	printer.Print([]logstore.LogLine{
		logstore.LogLine{Text: "layer 1: Partial\n", ProgressID: "layer 1", Time: now},
	})
	assert.Equal(t, "layer 1: Pending\nlayer 2: Pending\n", out.String())

	now = now.Add(time.Hour)
	printer.Print([]logstore.LogLine{
		logstore.LogLine{Text: "layer 1: Done\n", ProgressID: "layer 1", Time: now.Add(time.Hour)},
	})
	assert.Equal(t, "layer 1: Pending\nlayer 2: Pending\nlayer 1: Done\n", out.String())

}
func TestPrinterMustPrint(t *testing.T) {
	out := &bytes.Buffer{}
	now := time.Now()
	printer := NewIncrementalPrinter(Stdout(out))

	printer.Print([]logstore.LogLine{
		logstore.LogLine{Text: "layer 1: Pending\n", ProgressID: "layer 1", Time: now},
		logstore.LogLine{Text: "layer 2: Pending\n", ProgressID: "layer 2", Time: now},
	})

	assert.Equal(t, "layer 1: Pending\nlayer 2: Pending\n", out.String())

	printer.Print([]logstore.LogLine{
		logstore.LogLine{Text: "layer 1: Done\n", ProgressID: "layer 1", ProgressMustPrint: true, Time: now},
	})
	assert.Equal(t, "layer 1: Pending\nlayer 2: Pending\nlayer 1: Done\n", out.String())

}
