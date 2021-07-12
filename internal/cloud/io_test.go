package cloud

import (
	"bytes"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

func TestWriteSnapshotTo(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	buf := bytes.NewBuffer(nil)

	state := store.NewState()
	snapshot := &proto_webview.Snapshot{
		View: &proto_webview.View{
			UiSession: webview.ToUISession(*state),
		},
	}

	resources, err := webview.ToUIResourceList(*state)
	require.NoError(t, err)
	snapshot.View.UiResources = resources

	now := time.Unix(1551202573, 0)
	startTime, err := ptypes.TimestampProto(now)
	require.NoError(t, err)
	snapshot.View.TiltStartTime = startTime

	err = WriteSnapshotTo(ctx, snapshot, buf)
	assert.NoError(t, err)
	assert.Equal(t, `{
  "view": {
    "tiltStartTime": "2019-02-26T17:36:13Z",
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
          "updateStatus": "pending",
          "order": -1
        }
      }
    ]
  }
}
`, buf.String())
}
