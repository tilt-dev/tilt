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
COPY --from=gcr.io/windmill/foo:deadbeef /src/package.json /src/package.json
ADD . .
`, string(newDf))
	}
}

func TestInjectCopyFromWithLabel(t *testing.T) {
	df := Dockerfile(`
FROM golang:1.10
COPY --from=gcr.io/windmill/foo:bar /src/package.json /src/package.json
ADD . .
`)
	ref := container.MustParseNamedTagged("gcr.io/windmill/foo:deadbeef")
	newDf, modified, err := InjectImageDigest(df, ref)
	if assert.NoError(t, err) {
		assert.True(t, modified)
		assert.Equal(t, `
FROM golang:1.10
COPY --from=gcr.io/windmill/foo:deadbeef /src/package.json /src/package.json
ADD . .
`, string(newDf))
	}
}

func TestInjectCopyNormalizedNames(t *testing.T) {
	df := Dockerfile(`
FROM golang:1.10
COPY --from=vandelay/common /usr/src/common/package.json /usr/src/common/yarn.lock /usr/src/common/
ADD . .
`)
	ref := container.MustParseNamedTagged("vandelay/common:deadbeef")
	newDf, modified, err := InjectImageDigest(df, ref)
	if assert.NoError(t, err) {
		assert.True(t, modified)
		assert.Equal(t, `
FROM golang:1.10
COPY --from=docker.io/vandelay/common:deadbeef /usr/src/common/package.json /usr/src/common/yarn.lock /usr/src/common/
ADD . .
`, string(newDf))
	}
}

func TestInjectTwice(t *testing.T) {
	df := Dockerfile(`
FROM golang:1.10
COPY --from="vandelay/common" /usr/src/common/package.json /usr/src/common/yarn.lock
ADD . .
`)
	ref := container.MustParseNamedTagged("vandelay/common:deadbeef")
	ast, err := ParseAST(df)
	if err != nil {
		t.Fatal(err)
	}

	modified, err := ast.InjectImageDigest(ref)
	if assert.NoError(t, err) {
		assert.True(t, modified)

		newDf, err := ast.Print()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, `
FROM golang:1.10
COPY --from=docker.io/vandelay/common:deadbeef /usr/src/common/package.json /usr/src/common/yarn.lock
ADD . .
`, string(newDf))
	}

	modified, err = ast.InjectImageDigest(ref)
	if assert.NoError(t, err) {
		assert.True(t, modified)

		newDf, err := ast.Print()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, `
FROM golang:1.10
COPY --from=docker.io/vandelay/common:deadbeef /usr/src/common/package.json /usr/src/common/yarn.lock
ADD . .
`, string(newDf))
	}
}
