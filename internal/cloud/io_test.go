package cloud

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
)

func TestWriteSnapshotTo(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	buf := bytes.NewBuffer(nil)
	state := store.NewState()
	err := WriteSnapshotTo(ctx, *state, buf)
	assert.NoError(t, err)
	assert.Equal(t, `{
  "view": {
    "resources": [
      {
        "name": "(Tiltfile)",
        "lastDeployTime": "0001-01-01T00:00:00Z",
        "buildHistory": [
          {
            "startTime": "0001-01-01T00:00:00Z",
            "finishTime": "0001-01-01T00:00:00Z"
          }
        ],
        "currentBuild": {
          "startTime": "0001-01-01T00:00:00Z",
          "finishTime": "0001-01-01T00:00:00Z"
        },
        "runtimeStatus": "not_applicable",
        "updateStatus": "pending",
        "isTiltfile": true
      }
    ],
    "runningTiltBuild": {

    },
    "versionSettings": {
      "checkUpdates": true
    },
    "tiltCloudSchemeHost": "https:",
    "logList": {
      "fromCheckpoint": -1,
      "toCheckpoint": -1
    },
    "tiltStartTime": "0001-01-01T00:00:00Z"
  }
}
`, buf.String())
}
