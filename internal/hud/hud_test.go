package hud

import (
	"bytes"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gdamore/tcell"

	"github.com/windmilleng/tilt/internal/rty"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestRenderInit(t *testing.T) {
	logs := new(bytes.Buffer)
	ctx, _, ta := testutils.ForkedCtxAndAnalyticsForTest(logs)

	clockForTest := func() time.Time { return time.Date(2017, 1, 1, 12, 0, 0, 0, time.UTC) }
	r := NewRenderer(clockForTest)
	r.rty = rty.NewRTY(tcell.NewSimulationScreen(""), t)
	webURL, _ := url.Parse("http://localhost:10350")
	hud, err := ProvideHud(true, r, model.WebURL(*webURL), ta, NewIncrementalPrinter(logs))
	require.NoError(t, err)
	hud.(*Hud).refresh(ctx) // Ensure we render without error
}
