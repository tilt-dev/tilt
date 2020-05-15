package tiltfile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/testutils"

	"github.com/tilt-dev/tilt/internal/dockercompose"
)

// ParseConfig must return services topologically sorted wrt dependencies.

func TestParseConfigPreservesServiceOrder(t *testing.T) {
	f := newDCFixture(t)

	var output = `version: '3'
services:
  a:
    image: imga
  b:
    image: imgb
  c:
    image: imgc
  d:
    image: imgd
  e:
    image: imge
  f:
    image: imgf`

	var servicesOutput = `f
e
d
c
b
a`
	services := f.parse(output, servicesOutput)
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
	servicesOutput := `app
`
	services := f.parse(output, servicesOutput)
	if assert.Len(t, services, 1) {
		assert.Equal(t, []int{3000}, services[0].PublishedPorts)
	}

}

func TestPortMapSame(t *testing.T) {
	f := newDCFixture(t)

	output := `services:
  app:
    command: sh -c 'node server.js'
    image: tilt.dev/express-redis-app
    ports:
    - 3000/tcp
version: '3.0'
`
	servicesOutput := `app
`
	services := f.parse(output, servicesOutput)
	if assert.Len(t, services, 1) {
		assert.Equal(t, []int{3000}, services[0].PublishedPorts)
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
	servicesOutput := `app
`
	services := f.parse(output, servicesOutput)
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
	servicesOutput := `app
`
	services := f.parse(output, servicesOutput)
	if assert.Len(t, services, 1) {
		assert.Equal(t, []int{3000}, services[0].PublishedPorts)
	}
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

func (f dcFixture) parse(configOutput, servicesOutput string) []*dcService {
	f.dcCli.ConfigOutput = configOutput
	f.dcCli.ServicesOutput = servicesOutput

	services, err := parseDCConfig(f.ctx, f.dcCli, []string{"doesn't-matter.yml"})
	if err != nil {
		f.t.Fatalf("dcFixture.Parse: %v", err)
	}
	return services
}
