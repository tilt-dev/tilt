package store

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

func TestManifestTarget_FacetsSecretsScrubbed(t *testing.T) {
	m := model.Manifest{Name: "test_manifest"}.WithDeployTarget(model.K8sTarget{})
	mt := NewManifestTarget(m)

	s := "password1"
	b64 := base64.StdEncoding.EncodeToString([]byte(s))
	mt.State.BuildStatuses[m.DeployTarget().ID()] = &BuildStatus{
		LastResult: K8sBuildResult{AppliedEntitiesText: fmt.Sprintf("text %s moretext", b64)},
	}
	secrets := model.SecretSet{}
	secrets.AddSecret("foo", "password", []byte(s))
	actual := mt.Facets(logstore.NewLogStore(), secrets)
	expected := []model.Facet{
		{
			Name:  "applied yaml",
			Value: "text [redacted secret foo:password] moretext",
		},
	}

	require.Equal(t, expected, actual)
}
