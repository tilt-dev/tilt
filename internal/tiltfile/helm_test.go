package tiltfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmSetArgs(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupHelm()

	f.file("Tiltfile", `
yml = helm('./helm', name='rose-quartz', namespace='garnet', set=[
  'ingress.enabled=true',
  'service.externalPort=1234',
  'service.internalPort=5678'
])
k8s_yaml(yml)
`)

	f.load()

	m := f.assertNextManifestUnresourced("garnet",
		// A service and ingress with the same name
		"rose-quartz-helloworld-chart",
		"rose-quartz-helloworld-chart")
	yaml := m.K8sTarget().YAML

	// Set on the service
	assert.Contains(t, yaml, "port: 1234")
	assert.Contains(t, yaml, "targetPort: 5678")

	// Set on the ingress
	assert.Contains(t, yaml, "serviceName: rose-quartz-helloworld-chart")
	assert.Contains(t, yaml, "servicePort: 1234")
}
