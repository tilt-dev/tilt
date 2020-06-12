package hud

import (
	"bytes"
	"net/url"
	"testing"
	"time"

	"github.com/gdamore/tcell"

	"github.com/tilt-dev/tilt/internal/rty"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestRenderInit(t *testing.T) {
	logs := new(bytes.Buffer)
	ctx, _, ta := testutils.ForkedCtxAndAnalyticsForTest(logs)

	clockForTest := func() time.Time { return time.Date(2017, 1, 1, 12, 0, 0, 0, time.UTC) }
	r := NewRenderer(clockForTest)
	r.rty = rty.NewRTY(tcell.NewSimulationScreen(""), t)
	webURL, _ := url.Parse("http://localhost:10350")
	hud := NewHud(r, model.WebURL(*webURL), ta)
	hud.(*Hud).refresh(ctx) // Ensure we render without error
}
