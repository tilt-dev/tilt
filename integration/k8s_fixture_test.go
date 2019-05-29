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
)

type k8sFixture struct {
	*fixture

	token string
	cert  string

	oldKubeConfigPerms os.FileMode
	oldKubeConfigPath  string
	oldKubeConfig      string
}

func newK8sFixture(t *testing.T, dir string) *k8sFixture {
	f := newFixture(t, dir)

	kf := &k8sFixture{fixture: f}
	kf.CreateNamespaceIfNecessary()
	kf.createUser()
	kf.ClearNamespace()
	kf.getOldKubeConfig()
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

func (f *k8sFixture) CreateNamespaceIfNecessary() {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, "kubectl", "apply", "-f", "namespace.yaml")
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

func (f *k8sFixture) getOldKubeConfig() {
	usr, _ := user.Current()
	dir := usr.HomeDir
	// NOTE(dmiller): this assumes that your kube config exists in the default location, and is the one currently being used.
	// This is true in CI and on my machine, but is not universally true for anyone who might want to run this test on their machine.
	kubeConfigPath := filepath.Join(dir, ".kube", "config")

	old, err := ioutil.ReadFile(kubeConfigPath)
	if err != nil {
		f.t.Fatalf("Error reading KUBECONFIG: %v", err)
	}
	info, err := os.Stat(kubeConfigPath)
	if err != nil {
		f.t.Fatalf("Error stat'ing KUBECONFIG: %v", err)
	}

	f.oldKubeConfigPath = kubeConfigPath
	f.oldKubeConfigPerms = info.Mode()
	f.oldKubeConfig = string(old)
}

func (f *k8sFixture) maybeRestoreOldKubeConfig() {
	if f.oldKubeConfig != "" {
		err := ioutil.WriteFile(f.oldKubeConfigPath, []byte(f.oldKubeConfig), f.oldKubeConfigPerms)
		if err != nil {
			f.t.Fatalf("Error restoring old KUBECONFIG: %v", err)
		}
	}
}

func (f *k8sFixture) createUser() {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, "kubectl", "apply", "-f", "access.yaml")
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Error creating user: %v. Logs:\n%s", err, outWriter.String())
	}
}

func (f *k8sFixture) getSecrets() {
	outWriter := bytes.NewBuffer(nil)
	cmdStr := `kubectl get secrets -n tilt-integration -o json | jq -r '.items[] | select(.metadata.name | startswith("tilt-integration-user-token-")) | .data.token'`
	cmd := exec.CommandContext(f.ctx, "bash", "-c", cmdStr)
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	token, err := cmd.Output()
	if err != nil {
		f.t.Fatalf("Error getting secrets: %v. Cmd: %s", err, cmdStr)
	}

	cmdStr = `kubectl get secrets -n tilt-integration -o json | jq -r '.items[] | select(.metadata.name | startswith("tilt-integration-user-token-")) | .data["ca.crt"]'`
	cmd = exec.CommandContext(f.ctx, "bash", "-c", cmdStr)
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	cert, err := cmd.Output()
	if err != nil {
		f.t.Fatalf("Error getting secrets: %v. Cmd: %s", err, cmdStr)
	}

	f.token = string(token)
	f.cert = string(cert)
}

func (f *k8sFixture) SetRestrictedCredentials() {
	f.getSecrets()

	err := ioutil.WriteFile("/tmp/cert", []byte(f.cert), 0644)
	if err != nil {
		f.t.Fatalf("Error writing cert: %v", err)
	}
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, "kubectl", "config", "set-credentials", "tilt-integration-user", "--client-certificate=/tmp/cert", fmt.Sprintf("--token=%s", f.token))
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	err = cmd.Run()
	if err != nil {
		f.t.Fatalf("Error setting credentials: %v", err)
	}

	cmd = exec.CommandContext(f.ctx, "kubectl", "config", "current-context")
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	currentContext, err := cmd.Output()
	if err != nil {
		f.t.Fatalf("Error getting current context: %v", err)
	}

	cmd = exec.CommandContext(f.ctx, "kubectl", "config", "set-context", string(currentContext), "--user=tilt-integration-user")
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	err = cmd.Run()
	if err != nil {
		f.t.Fatalf("Error setting context user: %v", err)
	}

	cmdStr := fmt.Sprintf(`kubectl config view -o json | jq -r '.contexts[] | select(.name == "%s") | .context.cluster'`, strings.TrimSpace(string(currentContext)))
	cmd = exec.CommandContext(f.ctx, "bash", "-c", cmdStr)
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	currentCluster, err := cmd.Output()
	if err != nil {
		f.t.Fatalf("Error getting current cluster: %v. Cmd: %s", err, cmdStr)
	}

	cmd = exec.CommandContext(f.ctx, "kubectl", "config", "set-cluster", string(currentCluster), "--certificate-authority=/tmp/cert")
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	err = cmd.Run()
	if err != nil {
		f.t.Fatalf("Error setting cluster certificate: %v", err)
	}
}

func (f *k8sFixture) TearDown() {
	f.StartTearDown()
	f.ClearNamespace()
	f.maybeRestoreOldKubeConfig()
	f.fixture.TearDown()
}
