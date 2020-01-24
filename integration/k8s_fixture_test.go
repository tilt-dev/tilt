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
	f.WaitUntil(ctx, fmt.Sprintf("curl(%s)", url), func() (string, error) {
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
	for {
		allPodsReady, output, podNames := f.AllPodsInPhase(ctx, selector, phase)
		if allPodsReady {
			return podNames
		}

		select {
		case <-f.activeTiltDone():
			f.t.Fatalf("Tilt died while waiting for pods to be ready: %v", f.activeTiltErr())
		case <-ctx.Done():
			f.t.Fatalf("Timed out waiting for pods to be ready. Selector: %s. Output:\n:%s\n", selector, output)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Checks that all pods are in the given phase
// Returns the output (for diagnostics) and the name of the pods in the given phase.
func (f *k8sFixture) AllPodsInPhase(ctx context.Context, selector string, phase v1.PodPhase) (bool, string, []string) {
	cmd := exec.Command("kubectl", "get", "pods",
		namespaceFlag, "--selector="+selector, "-o=template",
		"--template", "{{range .items}}{{.metadata.name}} {{.status.phase}}{{println}}{{end}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatal(errors.Wrap(err, "get pods"))
	}

	outStr := string(out)
	lines := strings.Split(outStr, "\n")
	podNames := []string{}
	hasOneMatchingPod := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		elements := strings.Split(line, " ")
		if len(elements) < 2 {
			f.t.Fatalf("Unexpected output of kubect get pods: %s", outStr)
		}

		name, actualPhase := elements[0], elements[1]
		var matchedPhase bool
		if actualPhase == string(phase) {
			matchedPhase = true
			hasOneMatchingPod = true

		}

		if !matchedPhase {
			return false, outStr, nil
		}

		podNames = append(podNames, name)
	}
	return hasOneMatchingPod, outStr, podNames
}

func (f *k8sFixture) ForwardPort(name string, portMap string) {
	cmd := exec.CommandContext(f.ctx, "kubectl", "port-forward", namespaceFlag, name, portMap)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	err := cmd.Start()
	if err != nil {
		f.t.Fatal(err)
	}

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
	f.fixture.tilt.Environ["KUBECONFIG"] = f.kubeconfigPath
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

// waits for pods to be in a specified state, or times out and fails
type podWaiter struct {
	disallowedPodIDs map[string]bool
	f                *k8sFixture
	selector         string
	expectedPhase    v1.PodPhase
	expectedPodCount int // or -1 for no expectation
	timeout          time.Duration
}

func (f *k8sFixture) newPodWaiter(selector string) podWaiter {
	return podWaiter{
		f:                f,
		selector:         selector,
		expectedPodCount: -1,
		disallowedPodIDs: make(map[string]bool),
		timeout:          time.Minute,
	}
}

func (pw podWaiter) podPhases() (map[string]string, string) {
	cmd := exec.Command("kubectl", "get", "pods",
		namespaceFlag, "--selector="+pw.selector, "-o=template",
		"--template", "{{range .items}}{{.metadata.name}} {{.status.phase}}{{println}}{{end}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		pw.f.t.Fatal(errors.Wrap(err, "get pods"))
	}

	ret := make(map[string]string)

	outStr := string(out)
	lines := strings.Split(outStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		elements := strings.Split(line, " ")
		if len(elements) < 2 {
			pw.f.t.Fatalf("Unexpected output of kubect get pods: %s", outStr)
		}

		name, phase := elements[0], elements[1]

		ret[name] = phase
	}

	return ret, outStr
}

// if a test is transitioning from one pod id to another, it should disallow the old pod id here
// simply waiting for pod state does not suffice - it leaves us with a race condition:
// an old pod matching the label is up
// we run some command causing a new pod matching the label to come up
// while the deployment controller is swapping from the old pod to the new pod, there will be at least a moment
// in which both pods exist. we want to ensure we've made it past the point where the old pod is gone
func (pw podWaiter) withDisallowedPodIDs(podIDs []string) podWaiter {
	pw.disallowedPodIDs = make(map[string]bool)
	for _, podID := range podIDs {
		pw.disallowedPodIDs[podID] = true
	}
	return pw
}

func (pw podWaiter) withExpectedPodCount(count int) podWaiter {
	pw.expectedPodCount = count
	return pw
}

func (pw podWaiter) withExpectedPhase(phase v1.PodPhase) podWaiter {
	pw.expectedPhase = phase
	return pw
}

func (pw podWaiter) withTimeout(timeout time.Duration) podWaiter {
	pw.timeout = timeout
	return pw
}

func (pw podWaiter) wait() []string {
	ctx, cancel := context.WithTimeout(pw.f.ctx, pw.timeout)
	defer cancel()
	for {
		okToReturn := true
		podPhases, output := pw.podPhases()
		var podIDs []string
		for podID, phase := range podPhases {
			if (pw.expectedPhase != "" && string(pw.expectedPhase) != phase) || pw.disallowedPodIDs[podID] {
				okToReturn = false
				break
			}
			podIDs = append(podIDs, podID)
		}
		if okToReturn &&
			(pw.expectedPodCount == -1 || pw.expectedPodCount == len(podPhases)) {
			return podIDs
		}

		select {
		case <-pw.f.activeTiltDone():
			pw.f.t.Fatalf("Tilt died while waiting for pods to be ready: %v", pw.f.activeTiltErr())
		case <-ctx.Done():
			pw.f.t.Fatalf("Timed out waiting for pods to be ready. Selector: %s. Output:\n:%s\n", pw.selector, output)
		case <-time.After(200 * time.Millisecond):
		}
	}
}
