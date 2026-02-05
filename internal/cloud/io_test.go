package cloud

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/tiltfiles"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

func TestWriteSnapshotTo(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	buf := bytes.NewBuffer(nil)

	state := store.NewState()
	tiltfiles.HandleTiltfileUpsertAction(state, tiltfiles.TiltfileUpsertAction{
		Tiltfile: &v1alpha1.Tiltfile{
			ObjectMeta: metav1.ObjectMeta{Name: model.MainTiltfileManifestName.String()},
			Spec:       v1alpha1.TiltfileSpec{Path: "Tiltfile"},
		},
	})
	now := time.Unix(1551202573, 0)
	snapshot := &proto_webview.Snapshot{
		View: &proto_webview.View{
			UiSession: webview.ToUISession(*state),
		},
		CreatedAt: metav1.NewMicroTime(now),
	}

	resources, err := webview.ToUIResourceList(*state, make(map[string][]v1alpha1.DisableSource))
	require.NoError(t, err)
	snapshot.View.UiResources = resources

	for _, r := range resources {
		for i, cond := range r.Status.Conditions {
			// Clear the transition timestamps so that the test is hermetic.
			cond.LastTransitionTime = metav1.MicroTime{}
			r.Status.Conditions[i] = cond
		}
	}

	snapshot.View.TiltStartTime = metav1.NewMicroTime(now)

	err = WriteSnapshotTo(ctx, snapshot, buf)
	assert.NoError(t, err)
	assert.Equal(t, `{
  "view": {
    "tiltStartTime": "2019-02-26T17:36:13.000000Z",
    "uiSession": {
      "metadata": {
        "name": "Tiltfile"
      },
      "spec": {},
      "status": {
        "runningTiltBuild": {},
        "versionSettings": {
          "checkUpdates": true
        },
        "tiltStartTime": null,
        "tiltfileKey": "Tiltfile"
      }
    },
    "uiResources": [
      {
        "metadata": {
          "name": "(Tiltfile)"
        },
        "spec": {},
        "status": {
          "lastDeployTime": null,
          "pendingBuildSince": null,
          "runtimeStatus": "not_applicable",
          "updateStatus": "pending",
          "order": 1,
          "disableStatus": {
            "enabledCount": 0,
            "disabledCount": 0,
            "state": "Enabled",
            "sources": null
          },
          "conditions": [
            {
              "type": "UpToDate",
              "status": "False",
              "lastTransitionTime": null,
              "reason": "UpdatePending"
            },
            {
              "type": "Ready",
              "status": "False",
              "lastTransitionTime": null,
              "reason": "UpdatePending"
            }
          ]
        }
      }
    ]
  },
  "createdAt": "2019-02-26T17:36:13.000000Z"
}
`, buf.String())
}
