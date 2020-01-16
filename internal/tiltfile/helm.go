package tiltfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/ghodss/yaml"
)

// The helm template command outputs predictable yaml with a "Source:" comment,
// so take advantage of that.
const helmSeparator = "---\n"

const helmFileRepository = "file://"

var helmTestYAMLMatcher = regexp.MustCompile("^# Source: .*/tests/")

func filterHelmTestYAML(resourceBlob string) string {
	result := []string{}
	resources := strings.Split(resourceBlob, helmSeparator)
	for _, resource := range resources {
		if isHelmTestYAML(resource) {
			continue
		}

		result = append(result, resource)
	}
	return strings.Join(result, helmSeparator)
}

func isHelmTestYAML(resource string) bool {
	lines := strings.Split(resource, "\n")
	for _, line := range lines {
		if helmTestYAMLMatcher.MatchString(line) {
			return true
		}
	}
	return false
}

type helmVersion int

const (
	unknownHelmVersion helmVersion = iota
	helmV2
	helmV3
)

func parseVersion(version string) helmVersion {
	if strings.HasPrefix(version, "v3") {
		return helmV3
	} else if strings.HasPrefix(version, "Client: v2") {
		return helmV2
	}

	return unknownHelmVersion
}

func isHelmInstalled() bool {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("where", "helm.exe")
		if err := cmd.Run(); err != nil {
			return false
		}

		return true
	}

	cmd := exec.Command("/bin/sh", "-c", "command -v helm")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

func getHelmVersion() (helmVersion, error) {
	if !isHelmInstalled() {
		return unknownHelmVersion, unableToFindHelmErrorMessage()
	}

	// NOTE(dmiller): I pass `--client` here even though that doesn't do anything in Helm v3.
	// In Helm v2 that causes `helm version` to not reach out to tiller. Doing so can cause the
	// command to fail, even though Tilt doesn't use the server at all (it just calls
	// `helm template`).
	// In Helm v3, it has no effect, not even an unknown flag error.
	cmd := exec.Command("helm", "version", "--client", "--short")

	out, err := cmd.Output()
	if err != nil {
		return unknownHelmVersion, err
	}

	return parseVersion(string(out)), nil
}

func unableToFindHelmErrorMessage() error {
	var binaryName string
	if runtime.GOOS == "windows" {
		binaryName = "helm.exe"
	} else {
		binaryName = "helm"
	}

	return fmt.Errorf("Unable to find Helm installation. Make sure `%s` is on your $PATH.", binaryName)
}

func localSubchartDependenciesFromPath(chartPath string) ([]string, error) {
	var deps []string
	requirementsPath := filepath.Join(chartPath, "requirements.yaml")
	dat, err := ioutil.ReadFile(requirementsPath)
	if os.IsNotExist(err) {
		return deps, nil
	} else if err != nil {
		return deps, err
	}

	return localSubchartDependencies(dat)
}

type chartDependency struct {
	Repository string
}

type chartMetadata struct {
	Dependencies []chartDependency
}

func localSubchartDependencies(dat []byte) ([]string, error) {
	var deps []string
	var metadata chartMetadata

	err := yaml.Unmarshal([]byte(dat), &metadata)
	if err != nil {
		return deps, err
	}

	for _, d := range metadata.Dependencies {
		if strings.HasPrefix(d.Repository, helmFileRepository) {
			deps = append(deps, strings.TrimPrefix(d.Repository, helmFileRepository))
		}
	}

	return deps, nil
}
