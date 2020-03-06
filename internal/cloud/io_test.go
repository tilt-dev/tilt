package cloud

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
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
            "warnings": [
            ],
            "startTime": "0001-01-01T00:00:00Z",
            "finishTime": "0001-01-01T00:00:00Z",
            "updateTypes": [
            ]
          }
        ],
        "currentBuild": {
          "warnings": [
          ],
          "startTime": "0001-01-01T00:00:00Z",
          "finishTime": "0001-01-01T00:00:00Z",
          "updateTypes": [
          ]
        },
        "runtimeStatus": "ok",
        "isTiltfile": true
      }
    ],
    "featureFlags": {
    },
    "runningTiltBuild": {

    },
    "latestTiltBuild": {

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
