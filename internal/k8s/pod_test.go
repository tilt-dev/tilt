package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	expected := map[string][]PodID{
		"blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f": []PodID{PodID("blorg-fe-6b4477ffcd-xf98f")},
		"cockroachdb/cockroach:v2.0.5":                           []PodID{PodID("cockroachdb-0"), PodID("cockroachdb-1"), PodID("cockroachdb-2")},
	}
	assert.Equal(t, expected, podImgMap)
}
