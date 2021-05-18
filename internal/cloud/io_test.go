package cloud

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
)

func TestWriteSnapshotTo(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	buf := bytes.NewBuffer(nil)

	state := store.NewState()
	state.UISessions[types.NamespacedName{Name: webview.UISessionName}] = webview.ToUISession(*state)

	resources, err := webview.ToUIResourceList(*state)
	require.NoError(t, err)
	for _, r := range resources {
		state.UIResources[types.NamespacedName{Name: r.Name}] = r
	}

	err = WriteSnapshotTo(ctx, *state, buf)
	assert.NoError(t, err)
	assert.Equal(t, `{
  "view": {
    "logList": {
      "fromCheckpoint": -1,
      "toCheckpoint": -1
    },
    "tiltStartTime": "0001-01-01T00:00:00Z",
    "uiSession": {
      "metadata": {
        "name": "Tiltfile"
      },
      "status": {
        "versionSettings": {
          "checkUpdates": true
        },
        "tiltCloudSchemeHost": "https:"
      }
    },
    "uiResources": [
      {
        "metadata": {
          "name": "(Tiltfile)"
        },
        "status": {
          "runtimeStatus": "not_applicable",
          "updateStatus": "pending"
        }
      }
    ]
  }
}
`, buf.String())
}
