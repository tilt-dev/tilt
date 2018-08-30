package k8s

import (
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/magiconair/properties/assert"
)

var podsToImagesOut = `blorg-fe-6b4477ffcd-xf98f	blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f
cockroachdb-0	cockroachdb/cockroach:v2.0.5
cockroachdb-1	cockroachdb/cockroach:v2.0.5
cockroachdb-2	cockroachdb/cockroach:v2.0.5
`

func TestPodImgMapFromOutput(t *testing.T) {
	podImgMap, err := imgPodMapFromOutput(podsToImagesOut)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[reference.NamedTagged][]PodID{
		mustParseNamedTagged(t, "blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f"): []PodID{PodID("blorg-fe-6b4477ffcd-xf98f")},
		mustParseNamedTagged(t, "cockroachdb/cockroach:v2.0.5"):                           []PodID{PodID("cockroachdb-0"), PodID("cockroachdb-1"), PodID("cockroachdb-2")},
	}
	assert.Equal(t, expected, podImgMap)
}

func mustParseNamedTagged(t *testing.T, s string) reference.NamedTagged {
	nt, err := ParseNamedTagged(s)
	if err != nil {
		t.Fatal(err)
	}
	return nt
}
