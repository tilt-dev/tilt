//+build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

type k8sFixture struct {
	*fixture
	tempDir *tempdir.TempDirFixture

	token                 string
	cert                  string
	usingOverriddenConfig bool
}

func newK8sFixture(t *testing.T, dir string) *k8sFixture {
	f := newFixture(t, dir)
	td := tempdir.NewTempDirFixture(t)

	kf := &k8sFixture{fixture: f, tempDir: td}
	kf.CreateNamespaceIfNecessary()
	kf.createUser()
	kf.ClearNamespace()
	return kf
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
	outWriter := os.Stdout

	cmd := exec.CommandContext(f.ctx, "kubectl", "port-forward", namespaceFlag, name, portMap)
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	if f.usingOverriddenConfig {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", f.tempDir.JoinPath("config")))
	}
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
	if f.usingOverriddenConfig {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", f.tempDir.JoinPath("config")))
	}
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Error deleting deployments: %v. Logs:\n%s", err, outWriter.String())
	}
}

func (f *k8sFixture) CreateNamespaceIfNecessary() {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, "kubectl", "apply", "-f", "namespace.yaml")
	if f.usingOverriddenConfig {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", f.tempDir.JoinPath("config")))
	}
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Error creating namespace: %v. Logs:\n%s", err, outWriter.String())
	}
}

func (f *k8sFixture) ClearNamespace() {
	f.ClearResource("deployments")
	f.ClearResource("services")
}

func (f *k8sFixture) setupNewKubeConfig() {
	kubeConfigPath, exists := os.LookupEnv("KUBECONFIG")
	if !exists {
		usr, _ := user.Current()
		dir := usr.HomeDir
		kubeConfigPath = filepath.Join(dir, ".kube", "config")
	}

	current, err := ioutil.ReadFile(kubeConfigPath)
	if err != nil {
		f.t.Fatalf("Error reading KUBECONFIG: %v", err)
	}

	f.tempDir.WriteFile("config", string(current))
	f.usingOverriddenConfig = true
}

func (f *k8sFixture) createUser() {
	f.runCommandSilently("kubectl", "apply", "-f", "access.yaml")
}

func (f *k8sFixture) runCommandSilently(name string, arg ...string) {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, name, arg...)
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	if f.usingOverriddenConfig {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", f.tempDir.JoinPath("config")))
	}
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Error running command silently %s %v: %v", name, arg, err)
	}
}

func (f *k8sFixture) runCommandGetOutput(cmdStr string) string {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, "bash", "-c", cmdStr)
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	if f.usingOverriddenConfig {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", f.tempDir.JoinPath("config")))
	}
	output, err := cmd.Output()
	if err != nil {
		f.t.Fatalf("Error running command with output %s: %v", cmdStr, err)
	}

	return strings.TrimSpace(string(output))
}

func (f *k8sFixture) getSecrets() {
	cmdStr := `kubectl get secrets -n tilt-integration -o json | jq -r '.items[] | select(.metadata.name | startswith("tilt-integration-user-token-")) | .data.token'`
	token := f.runCommandGetOutput(cmdStr)

	cmdStr = `kubectl get secrets -n tilt-integration -o json | jq -r '.items[] | select(.metadata.name | startswith("tilt-integration-user-token-")) | .data["ca.crt"]'`
	cert := f.runCommandGetOutput(cmdStr)

	f.token = token
	f.cert = cert
}

func (f *k8sFixture) SetRestrictedCredentials() {
	f.setupNewKubeConfig()
	f.createUser()
	f.getSecrets()
	f.fixture.tiltEnviron["KUBECONFIG"] = f.tempDir.JoinPath("config")

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
