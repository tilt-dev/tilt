package tiltfile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// ParseConfig must return services topologically sorted wrt dependencies.

func TestParseConfigPreservesServiceOrder(t *testing.T) {
	f := newDCFixture(t)

	var output = `version: '3'
services:
  a:
    image: imga
    depends_on: [b]
  b:
    image: imgb
    depends_on: [c]
  c:
    image: imgc
    depends_on: [d, e, f]
  d:
    image: imgd
    depends_on: [f, e]
  e:
    image: imge
    depends_on: [f]
  f:
    image: imgf
`

	services := f.parse(output)
	if assert.Len(t, services, 6) {
		for i, name := range []string{"f", "e", "d", "c", "b", "a"} {
			assert.Equal(t, name, services[i].Name)
		}
	}
}

func TestPortStruct(t *testing.T) {
	f := newDCFixture(t)

	output := `services:
  app:
    command: sh -c 'node server.js'
    image: tilt.dev/express-redis-app
    ports:
    - published: 3000
      target: 30
version: '3.2'
`
	services := f.parse(output)
	if assert.Len(t, services, 1) {
		assert.Equal(t, []int{3000}, services[0].PublishedPorts)
	}

}

func TestPortMapRandomized(t *testing.T) {
	f := newDCFixture(t)

	output := `services:
  app:
    command: sh -c 'node server.js'
    image: tilt.dev/express-redis-app
    ports:
    - 3000/tcp
version: '3.0'
`
	services := f.parse(output)
	if assert.Len(t, services, 1) {
		assert.Empty(t, services[0].PublishedPorts)
	}
}

func TestPortMapDifferent(t *testing.T) {
	f := newDCFixture(t)

	output := `services:
  app:
    command: sh -c 'node server.js'
    image: tilt.dev/express-redis-app
    ports:
    - 3000:30/tcp
version: '3.0'
`

	services := f.parse(output)
	if assert.Len(t, services, 1) {
		assert.Equal(t, []int{3000}, services[0].PublishedPorts)
	}
}

func TestPortMapIP(t *testing.T) {
	f := newDCFixture(t)

	output := `services:
  app:
    command: sh -c 'node server.js'
    image: tilt.dev/express-redis-app
    ports:
    - 127.0.0.1:3000:30/tcp
version: '3.0'
`

	services := f.parse(output)
	if assert.Len(t, services, 1) {
		assert.Equal(t, []int{3000}, services[0].PublishedPorts)
	}
}

func TestMarshalOverflow(t *testing.T) {
	f := newDCFixture(t)

	// certain Compose types cause a stack overflow panic if marshaled with gopkg.in/yaml.v3
	// https://github.com/tilt-dev/tilt/issues/4797
	output := `services:
  foo:
    image: myimage
    ulimits:
      nproc: 65535
      nofile:
        soft: 20000
        hard: 40000
`

	services := f.parse(output)
	assert.NotEmpty(t, services)
}

type dcFixture struct {
	t     *testing.T
	ctx   context.Context
	dcCli *dockercompose.FakeDCClient
}

func newDCFixture(t *testing.T) dcFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	dcCli := dockercompose.NewFakeDockerComposeClient(t, ctx)
	return dcFixture{
		t:     t,
		ctx:   ctx,
		dcCli: dcCli,
	}
}

func (f dcFixture) parse(configOutput string) []*dcService {
	f.t.Helper()

	f.dcCli.ConfigOutput = configOutput

	services, err := parseDCConfig(f.ctx, f.dcCli, v1alpha1.DockerComposeProject{ConfigPaths: []string{"doesn't-matter.yml"}})
	if err != nil {
		f.t.Fatalf("dcFixture.Parse: %v", err)
	}
	return services
}
