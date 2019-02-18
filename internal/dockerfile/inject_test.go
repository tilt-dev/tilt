package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/container"
)

func TestInjectUntagged(t *testing.T) {
	df := Dockerfile(`
FROM gcr.io/windmill/foo
ADD . .
`)
	ref := container.MustParseNamedTagged("gcr.io/windmill/foo:deadbeef")
	newDf, modified, err := InjectImageDigest(df, ref)
	if assert.NoError(t, err) {
		assert.True(t, modified)
		assert.Equal(t, `
FROM gcr.io/windmill/foo:deadbeef
ADD . .
`, string(newDf))
	}
}

func TestInjectTagged(t *testing.T) {
	df := Dockerfile(`
FROM gcr.io/windmill/foo:v1
ADD . .
`)
	ref := container.MustParseNamedTagged("gcr.io/windmill/foo:deadbeef")
	newDf, modified, err := InjectImageDigest(df, ref)
	if assert.NoError(t, err) {
		assert.True(t, modified)
		assert.Equal(t, `
FROM gcr.io/windmill/foo:deadbeef
ADD . .
`, string(newDf))
	}
}

func TestInjectNoMatch(t *testing.T) {
	df := Dockerfile(`
FROM gcr.io/windmill/bar:v1
ADD . .
`)
	ref := container.MustParseNamedTagged("gcr.io/windmill/foo:deadbeef")
	newDf, modified, err := InjectImageDigest(df, ref)
	if assert.NoError(t, err) {
		assert.False(t, modified)
		assert.Equal(t, df, newDf)
	}
}

func TestInjectCopyFrom(t *testing.T) {
	df := Dockerfile(`
FROM golang:1.10
COPY --from=gcr.io/windmill/foo /src/package.json /src/package.json
ADD . .
`)
	ref := container.MustParseNamedTagged("gcr.io/windmill/foo:deadbeef")
	newDf, modified, err := InjectImageDigest(df, ref)
	if assert.NoError(t, err) {
		assert.True(t, modified)
		assert.Equal(t, `
FROM golang:1.10
COPY --from="gcr.io/windmill/foo:deadbeef" /src/package.json /src/package.json
ADD . .
`, string(newDf))
	}
}

func TestInjectCopyFromWithLabel(t *testing.T) {
	df := Dockerfile(`
FROM golang:1.10
COPY --from="gcr.io/windmill/foo:bar" /src/package.json /src/package.json
ADD . .
`)
	ref := container.MustParseNamedTagged("gcr.io/windmill/foo:deadbeef")
	newDf, modified, err := InjectImageDigest(df, ref)
	if assert.NoError(t, err) {
		assert.True(t, modified)
		assert.Equal(t, `
FROM golang:1.10
COPY --from="gcr.io/windmill/foo:deadbeef" /src/package.json /src/package.json
ADD . .
`, string(newDf))
	}
}
