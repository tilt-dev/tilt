//+build integration

package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

var k8sInstalled bool

type k8sFixture struct {
	*fixture
	tempDir *tempdir.TempDirFixture

	token          string
	cert           string
	kubeconfigPath string
}

func newK8sFixture(t *testing.T, dir string) *k8sFixture {
	f := newFixture(t, dir)
	td := tempdir.NewTempDirFixture(t)

	kf := &k8sFixture{fixture: f, tempDir: td}

	if !k8sInstalled {
		kf.checkKubectlConnection()

		// Delete the namespace when the test starts,
		// to make sure nothing is left over from previous tests.
		kf.deleteNamespace()

		k8sInstalled = true
	} else {
		kf.ClearNamespace()
	}

	return kf
}

func (f *k8sFixture) checkKubectlConnection() {
	cmd := exec.CommandContext(f.ctx, "kubectl", "version")
	f.runOrFail(cmd, "Checking kubectl connection")
}

func (f *k8sFixture) deleteNamespace() {
	cmd := exec.CommandContext(f.ctx, "kubectl", "delete", "namespace", "tilt-integration", "--ignore-not-found")
	f.runOrFail(cmd, "Deleting namespace tilt-integration")

	// block until the namespace doesn't exist, since kubectl often returns and the namespace is still "terminating"
	// which causes the creation of objects in that namespace to fail
	var b []byte
	args := []string{"kubectl", "get", "namespace", "tilt-integration", "--ignore-not-found"}
	timeout := time.Now().Add(10 * time.Second)
	for time.Now().Before(timeout) {
		cmd := exec.CommandContext(f.ctx, args[0], args[1:]...)
		b, err := cmd.Output()
		if err != nil {
			f.t.Fatalf("Error: checking that deletion of the tilt-integration namespace has completed: %v", err)
		}
		if len(b) == 0 {
			return
		}
	}
	f.t.Fatalf("timed out waiting for tilt-integration deletion to complete. last output of %q: %q", args, string(b))
}

func (f *k8sFixture) Curl(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrap(err, "Curl")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		f.t.Errorf("Error fetching %s: %s", url, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Curl")
	}
	return string(body), nil
}

func (f *k8sFixture) CurlUntil(ctx context.Context, url string, expectedContents string) {
	f.WaitUntilContains(ctx, fmt.Sprintf("curl(%s)", url), func() (string, error) {
		return f.Curl(url)
	}, expectedContents)
}

// Waits until all pods matching the selector are ready (i.e. phase = "Running")
// At least one pod must match.
// Returns the names of the ready pods.
func (f *k8sFixture) WaitForAllPodsReady(ctx context.Context, selector string) []string {
	return f.WaitForAllPodsInPhase(ctx, selector, v1.PodRunning)
}

func (f *k8sFixture) WaitForAllPodsInPhase(ctx context.Context, selector string, phase v1.PodPhase) []string {
	msg := fmt.Sprintf("All pods (selector=%q) in phase %s", selector, phase)
	podNames := []string{}
	f.WaitUntil(ctx, msg, func() (bool, error) {
		var allPodsInPhase bool
		allPodsInPhase, podNames = f.AllPodsInPhase(ctx, selector, phase)
		return allPodsInPhase, nil
	})
	return podNames
}

// Returns a map of pod names to their phase names
func (f *k8sFixture) PodPhases(ctx context.Context, selector string) map[string]string {
	cmd := exec.Command("kubectl", "get", "pods",
		namespaceFlag, "--selector="+selector, "-o=template",
		"--template", "{{range .items}}{{.metadata.name}}|{{.status.phase}}|{{.metadata.deletionTimestamp}}{{println}}{{end}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatal(errors.Wrap(err, "get pods"))
	}

	outStr := string(out)
	lines := strings.Split(outStr, "\n")

	result := map[string]string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		elements := strings.Split(line, "|")
		if len(elements) != 3 {
			f.t.Fatalf("Unexpected output of kubectl get pods: %s", outStr)
		}

		isDeleting := elements[2] != "<no value>"
		if isDeleting {
			// Ignore elements that are being deleted
			continue
		}

		result[elements[0]] = elements[1]
	}
	return result
}

// Return at least one pod in the given phase
func (f *k8sFixture) SomePodsInPhase(ctx context.Context, selector string, phase v1.PodPhase) (bool, []string) {
	podPhases := f.PodPhases(ctx, selector)
	podNames := []string{}
	for name, actualPhase := range podPhases {
		if actualPhase != string(phase) {
			continue
		}
		podNames = append(podNames, name)
	}
	return len(podNames) > 0, podNames
}

// Checks that all pods are in the given phase
// Returns the output (for diagnostics) and the name of the pods in the given phase.
func (f *k8sFixture) AllPodsInPhase(ctx context.Context, selector string, phase v1.PodPhase) (bool, []string) {
	podPhases := f.PodPhases(ctx, selector)
	podNames := []string{}
	for name, actualPhase := range podPhases {
		if actualPhase != string(phase) {
			return false, nil
		}
		podNames = append(podNames, name)
	}
	return len(podNames) > 0, podNames
}

func (f *k8sFixture) WaitForAllContainersForPodReady(ctx context.Context, pod string) {
	f.WaitUntil(ctx, fmt.Sprintf("All containers of pod ready %s", pod), func() (bool, error) {
		return f.AllContainersForPodReady(ctx, pod), nil
	})
}

// Checks that all containers for the given pod are ready
func (f *k8sFixture) AllContainersForPodReady(ctx context.Context, pod string) bool {
	cmd := exec.Command("kubectl", "get", "pod", pod,
		namespaceFlag, "-o=template",
		"--template", "{{range .status.containerStatuses}}{{.ready}}{{println}}{{end}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatal(errors.Wrapf(err, "get pod %s", pod))
	}

	outStr := strings.TrimSpace(string(out))
	lines := strings.Split(outStr, "\n")
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		if line != "true" {
			return false
		}
	}
	return true
}

func (f *k8sFixture) ForwardPort(name string, portMap string) {
	cmd := exec.CommandContext(f.ctx, "kubectl", "port-forward", namespaceFlag, name, portMap)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	err := cmd.Start()
	if err != nil {
		f.t.Fatal(err)
	}

	f.cmds = append(f.cmds, cmd)
	go func() {
		err := cmd.Wait()
		if err != nil && !f.tearingDown {
			fmt.Printf("port forward failed: %v\n", err)
		}
	}()
}

func (f *k8sFixture) ClearResource(name string) {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, "kubectl", "delete", name, namespaceFlag, "--all")
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Error deleting deployments: %v. Logs:\n%s", err, outWriter.String())
	}
}

func (f *k8sFixture) ClearNamespace() {
	f.ClearResource("jobs")
	f.ClearResource("deployments")
	f.ClearResource("services")
}

func (f *k8sFixture) setupNewKubeConfig() {
	cmd := exec.CommandContext(f.ctx, "kubectl", "config", "view", "--minify")
	current, err := cmd.Output()
	if err != nil {
		f.t.Fatalf("Error reading KUBECONFIG: %v", err)
	}

	// Create a file with the same basename as the current kubeconfig,
	// because we sometimes use that for env detection.
	kubeconfigBaseName := filepath.Base(os.Getenv("KUBECONFIG"))
	if kubeconfigBaseName == "" || kubeconfigBaseName == "." {
		kubeconfigBaseName = "config"
	}
	f.kubeconfigPath = f.tempDir.JoinPath(kubeconfigBaseName)
	f.tempDir.WriteFile(f.kubeconfigPath, string(current))
	f.fixture.tiltEnviron["KUBECONFIG"] = f.kubeconfigPath
	log.Printf("New kubeconfig: %s", f.kubeconfigPath)
}

func (f *k8sFixture) runCommand(name string, arg ...string) (*bytes.Buffer, error) {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, name, arg...)
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	if f.kubeconfigPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", f.kubeconfigPath))
	}
	err := cmd.Run()
	return outWriter, err
}

func (f *k8sFixture) runCommandSilently(name string, arg ...string) {
	_, err := f.runCommand(name, arg...)
	if err != nil {
		f.t.Fatalf("Error running command silently %s %v: %v", name, arg, err)
	}
}

func (f *k8sFixture) runCommandGetOutput(cmdStr string) string {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, "bash", "-c", cmdStr)
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	if f.kubeconfigPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", f.kubeconfigPath))
	}
	output, err := cmd.Output()
	if err != nil {
		f.t.Fatalf("Error running command with output %s: %v", cmdStr, err)
	}

	return strings.TrimSpace(string(output))
}

func (f *k8sFixture) getSecrets() {
	cmdStr := `kubectl get secrets -n tilt-integration -o json | jq -r '.items[] | select(.metadata.name | startswith("tilt-integration-user-token-")) | .data.token'`
	tokenBase64 := f.runCommandGetOutput(cmdStr)
	tokenBytes, err := base64.StdEncoding.DecodeString(tokenBase64)
	if err != nil {
		f.t.Fatalf("Unable to decode token: %v", err)
	}

	cmdStr = `kubectl get secrets -n tilt-integration -o json | jq -r '.items[] | select(.metadata.name | startswith("tilt-integration-user-token-")) | .data["ca.crt"]'`
	cert := f.runCommandGetOutput(cmdStr)

	f.token = string(tokenBytes)
	f.cert = cert
}

func (f *k8sFixture) SetRestrictedCredentials() {
	// docker-for-desktop has a default binding that gives service accounts access to everything.
	// See: https://github.com/docker/for-mac/issues/3694
	f.runCommandSilently("kubectl", "delete", "clusterrolebinding", "docker-for-desktop-binding", "--ignore-not-found")

	// The service account needs the namespace to exist.
	f.runCommandSilently("kubectl", "apply", "-f", "namespace.yaml")
	f.runCommandSilently("kubectl", "apply", "-f", "service-account.yaml")
	f.runCommandSilently("kubectl", "apply", "-f", "access.yaml")
	f.getSecrets()

	f.setupNewKubeConfig()

	f.runCommandSilently("kubectl", "config", "set-credentials", "tilt-integration-user", fmt.Sprintf("--token=%s", f.token))
	f.runCommandSilently("kubectl", "config", "set", "users.tilt-integration-user.client-key-data", f.cert)

	currentContext := f.runCommandGetOutput("kubectl config current-context")

	f.runCommandSilently("kubectl", "config", "set-context", currentContext, "--user=tilt-integration-user", "--namespace=tilt-integration")

	cmdStr := fmt.Sprintf(`kubectl config view -o json | jq -r '.contexts[] | select(.name == "%s") | .context.cluster'`, currentContext)
	currentCluster := f.runCommandGetOutput(cmdStr)

	f.runCommandSilently("kubectl", "config", "set", fmt.Sprintf("clusters.%s.certificate-authority-data", currentCluster), f.cert)
	f.runCommandSilently("kubectl", "config", "unset", fmt.Sprintf("clusters.%s.certificate-authority", currentCluster))
}

func (f *k8sFixture) TearDown() {
	f.StartTearDown()
	f.ClearNamespace()
	f.fixture.TearDown()
	f.tempDir.TearDown()
}
