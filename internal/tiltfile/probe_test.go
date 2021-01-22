package tiltfile

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestProbeMetaOptions(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
p = probe(initial_delay_secs=123,
          timeout_secs=456,
          period_secs=789,
          success_threshold=987,
          failure_threshold=654)

local_resource('test', serve_cmd='echo "Hi"', readiness_probe=p)
`)

	f.load()

	f.assertNumManifests(1)

	probeSpec := &v1.Probe{
		InitialDelaySeconds: 123,
		TimeoutSeconds:      456,
		PeriodSeconds:       789,
		SuccessThreshold:    987,
		FailureThreshold:    654,
	}

	f.assertNextManifest("test", localTarget(readinessProbeHelper{probeSpec}))
}

func TestProbeExec(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
p = probe(exec=exec_action(command=["sleep", "60"]))

local_resource('test', serve_cmd='echo "Hi"', readiness_probe=p)
`)

	f.load()

	f.assertNumManifests(1)

	probeSpec := &v1.Probe{
		Handler: v1.Handler{Exec: &v1.ExecAction{
			Command: []string{"sleep", "60"},
		}},
	}

	f.assertNextManifest("test", localTarget(readinessProbeHelper{probeSpec}))
}

func TestProbeHTTPGet(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
p = probe(http_get=http_get_action(host="example.com", port=8888, scheme='https', path='/status'))

local_resource('test', serve_cmd='echo "Hi"', readiness_probe=p)
`)

	f.load()

	f.assertNumManifests(1)

	probeSpec := &v1.Probe{
		Handler: v1.Handler{HTTPGet: &v1.HTTPGetAction{
			Host:   "example.com",
			Port:   intstr.FromInt(8888),
			Scheme: v1.URISchemeHTTPS,
			Path:   "/status",
		}},
	}

	f.assertNextManifest("test", localTarget(readinessProbeHelper{probeSpec}))
}

func TestProbeTCP(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
p = probe(tcp_socket=tcp_socket_action("localhost", 1234))

local_resource('test', serve_cmd='echo "Hi"', readiness_probe=p)
`)

	f.load()

	f.assertNumManifests(1)

	probeSpec := &v1.Probe{
		Handler: v1.Handler{TCPSocket: &v1.TCPSocketAction{
			Host: "localhost",
			Port: intstr.FromInt(1234),
		}},
	}

	f.assertNextManifest("test", localTarget(readinessProbeHelper{probeSpec}))
}
