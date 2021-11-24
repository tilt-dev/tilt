package uiresources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func resourceWithDisableCount(count int) *v1alpha1.UIResource {
	return &v1alpha1.UIResource{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Status: v1alpha1.UIResourceStatus{
			DisableStatus: v1alpha1.DisableResourceStatus{DisabledCount: int32(count)},
		},
	}
}

func TestLogging(t *testing.T) {
	for _, tc := range []struct {
		name        string
		old, new    *v1alpha1.UIResource
		expectedLog string
	}{
		{"enable", resourceWithDisableCount(1), resourceWithDisableCount(0), "Resource \"foo\" enabled."},
		{"disable", resourceWithDisableCount(0), resourceWithDisableCount(1), "Resource \"foo\" disabled."},
		{"old nil", nil, resourceWithDisableCount(0), ""},
		{"enabled, no change", resourceWithDisableCount(0), resourceWithDisableCount(0), ""},
		{"disabled, no change", resourceWithDisableCount(1), resourceWithDisableCount(1), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := store.NewState()
			state.UIResources["foo"] = tc.old

			action := UIResourceUpsertAction{UIResource: tc.new}

			HandleUIResourceUpsertAction(state, action)

			log := state.LogStore.ManifestLog("foo")
			if tc.expectedLog == "" {
				require.Equal(t, "", log)
			} else {
				lines := strings.Split(log, "\n")
				require.Equal(t, 2, len(lines))
				require.Contains(t, lines[0], tc.expectedLog)
				require.Equal(t, "", lines[1])
			}
		})
	}
}
