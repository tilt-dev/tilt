//go:build integration
// +build integration

package integration

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type localResource struct {
	name        string
	triggerMode string
	autoInit    bool
	updateCmd   string
	serveCmd    string
}

// TestTiltCI covers a variety of different permutations for local_resource + k8s_resource, in particular around
// auto_init and absence/presence of update cmd/serve_cmd (for local_resource).
//
// Critically, it ensures that `tilt ci` does not block indefinitely; there have been several regressions around
// this due to the subtlety in various states.
func TestTiltCI(t *testing.T) {
	f := newK8sFixture(t, "tilt_ci")
	f.SetRestrictedCredentials()

	// dynamically generate a bunch of combinations of local_resource args and write out to
	// `Tiltfile.generated` which is loaded in by the main/static Tiltfile (which contains
	// hardcoded K8s definitions as there's fewer combinations there)
	localResources := generateLocalResources()
	generateTiltfile(f.fixture, localResources)

	f.TiltCI()

	// NOTE: the assertions don't use assert.Contains because on failure it'll re-print all the logs
	// 	which have already been printed and make reading the test failure very difficult
	logs := f.logs.String()
	for _, lr := range localResources {
		if !lr.autoInit {
			assert.Falsef(t, strings.Contains(logs, lr.name),
				"Resource %q had auto_init=False and should not have been seen", lr.name)
			continue
		}

		if lr.updateCmd != "" {
			assert.Truef(t, strings.Contains(logs, fmt.Sprintf("update for %s", lr.name)),
				"Resource %q did not log via update_cmd", lr.name)
		}

		if lr.serveCmd != "" {
			assert.Truef(t, strings.Contains(logs, fmt.Sprintf("serve for %s", lr.name)),
				"Resource %q did not log via serve_cmd", lr.name)
		}
	}

	for _, name := range []string{"k8s-server-disabled", "k8s-job-disabled"} {
		assert.Falsef(t, strings.Contains(logs, fmt.Sprintf("Initial Build • %s", name)),
			"Resource %q had auto_init=False and should not have been deployed", name)
	}

	assert.True(t, strings.Contains(logs, "k8s-server-e… │ Initial Build"),
		"Resource k8s-server-enabled did not deploy")
	assert.True(t, strings.Contains(logs, "k8s-job-enab… │ Initial Build"),
		"Resource k8s-job-enabled did not deploy")

	assert.True(t, strings.Contains(logs, "k8s-job-enabled finished"),
		`Resource "k8s-job-enabled" did not log via container`)

	assert.True(t, strings.Contains(logs, "k8s-server-enabled running"),
		`Resource "k8s-server-enabled" did not log via container`)
}

func generateTiltfile(f *fixture, localResources []localResource) {
	var out bytes.Buffer
	out.WriteString("# AUTOMATICALLY GENERATED - DO NOT EDIT\n")
	out.WriteString("# Run TestTiltCI to re-generate\n\n")

	for _, lr := range localResources {
		out.WriteString(lr.String())
		out.WriteString("\n")
	}

	err := os.WriteFile(f.testDirPath("Tiltfile.generated"), out.Bytes(), os.FileMode(0777))
	require.NoError(f.t, err, "Failed to write Tiltfile.generated")
}

func generateLocalResources() []localResource {
	var localResources []localResource
	for _, tm := range []string{"TRIGGER_MODE_AUTO", "TRIGGER_MODE_MANUAL"} {
		for _, autoInit := range []bool{false, true} {
			for _, hasUpdateCmd := range []bool{false, true} {
				for _, hasServeCmd := range []bool{false, true} {
					if !hasUpdateCmd && !hasServeCmd {
						continue
					}
					lr := localResource{triggerMode: tm, autoInit: autoInit}
					lr.name = "local"
					if lr.triggerMode == "TRIGGER_MODE_MANUAL" {
						lr.name += "-trigger_manual"
					}
					if !autoInit {
						lr.name += "-no_auto_init"
					}
					if hasUpdateCmd {
						lr.name += "-update_cmd"
					}
					if hasServeCmd {
						lr.name += "-serve_cmd"
					}

					// we use the names in the commands so need to finish building the name before setting them
					if hasUpdateCmd {
						lr.updateCmd = fmt.Sprintf(`echo "update for %s"`, lr.name)
					}
					if hasServeCmd {
						lr.serveCmd = fmt.Sprintf(`while true; do echo "serve for %s"; sleep 5000; done`, lr.name)
					}

					localResources = append(localResources, lr)
				}
			}
		}
	}
	return localResources
}

func (lr localResource) String() string {
	var args []string
	args = append(args, fmt.Sprintf("name=%q", lr.name))
	var autoInitArgVal string
	if lr.autoInit {
		autoInitArgVal = "True"
	} else {
		autoInitArgVal = "False"
	}
	args = append(args, fmt.Sprintf("auto_init=%s", autoInitArgVal))
	args = append(args, fmt.Sprintf("trigger_mode=%s", lr.triggerMode))
	if lr.updateCmd != "" {
		args = append(args, fmt.Sprintf("cmd=%q", lr.updateCmd))
	}
	if lr.serveCmd != "" {
		args = append(args, fmt.Sprintf("serve_cmd=%q", lr.serveCmd))
	}

	var out strings.Builder
	out.WriteString(`local_resource(`)
	for i, arg := range args {
		out.WriteString(arg)
		if i != len(args)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(")")
	return out.String()
}
