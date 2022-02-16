package rancher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/adrg/xdg"

	"github.com/tilt-dev/tilt/pkg/logger"
)

type ContainerRuntime int

const (
	ContainerRuntimeUnknown ContainerRuntime = iota
	ContainerRuntimeContainerd
	ContainerRuntimeDocker
)

// openFileFunc is a hack for testing.
type openFileFunc func(name string) (io.ReadCloser, error)

var openFile = os.Open

// settings we are interested in from Rancher Desktop.
//
// See https://github.com/rancher-sandbox/rancher-desktop/blob/aa1bb53340e971083b74d990ed6daeeceebd3bfe/src/config/settings.ts
type settings struct {
	Version    int `json:"version"`
	Kubernetes struct {
		ContainerEngine string `json:"containerEngine"`
	} `json:"kubernetes"`
}

// DetermineContainerRuntime reads the Rancher Desktop settings and returns the
// configured container runtime (e.g. containerd, moby/docker).
//
// Tilt uses this to determine if it's building directly to the container
// runtime so that it can skip registry pushes.
//
// No attempt is made to determine that Rancher Desktop is actively running;
// it's assumed that Tilt has already determined it's using a Rancher Desktop
// Kubernetes context/connection.
//
// All failures (e.g. config file does not exist, the config file cannot be
// parsed, etc) will be logged at DEBUG level and ContainerRuntimeUnknown
// returned. Where possible, the log message will include the version of the
// Rancher Desktop settings file: it's incremented on breaking change, so this
// can help diagnose the issue. In the future, it might be useful to alert on
// failures where this is greater than a known "good" version, as that's likely
// indicative of the need for updates here to handle it.
//
// See MinikubeClient::DockerEnv for similar functionality for minikube.
func DetermineContainerRuntime(ctx context.Context) ContainerRuntime {
	cfgDir, err := configDirPath()
	if err != nil {
		logger.Get(ctx).Debugf(
			"Failed to determine Rancher Desktop config file path: %v",
			err)
		return ContainerRuntimeUnknown
	}
	cfgPath := filepath.Join(cfgDir, "settings.json")

	f, err := openFile(cfgPath)
	if err != nil {
		// TODO(milas): maybe we should surface errors to the user with
		// 	a nice message about how things might misbehave since we can't
		// 	infer the container runtime?
		logger.Get(ctx).Debugf(
			"Failed to read Rancher Desktop config: %v", err)
		return ContainerRuntimeUnknown
	}
	defer func() {
		_ = f.Close()
	}()

	var cfg settings
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		logger.Get(ctx).Debugf(
			"Failed to parse Rancher Desktop config at %q: %v",
			cfgDir, err)
		return ContainerRuntimeUnknown
	}

	switch strings.TrimSpace(cfg.Kubernetes.ContainerEngine) {
	case "containerd":
		return ContainerRuntimeContainerd
	case "moby":
		return ContainerRuntimeDocker
	case "":
		logger.Get(ctx).Debugf(
			"No container runtime specified in config (version: %d)",
			cfg.Version)
		return ContainerRuntimeUnknown
	default:
		logger.Get(ctx).Debugf(
			"Unknown container runtime %q specified in config (version: %d)",
			cfg.Kubernetes.ContainerEngine, cfg.Version)
		return ContainerRuntimeUnknown
	}
}

// configDirPath returns the path to the Rancher Desktop configuration directory.
//
// See https://github.com/rancher-sandbox/rancher-desktop/blob/aa1bb53340e971083b74d990ed6daeeceebd3bfe/src/utils/paths.ts
func configDirPath() (string, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home dir: %v", err)
	}

	const appName = "rancher-desktop"
	switch runtime.GOOS {
	case "windows":
		appDataPath := os.Getenv("APPDATA")
		if appDataPath == "" {
			appDataPath = filepath.Join(homePath, "AppData", "Roaming")
		}
		return filepath.Join(appDataPath, appName), nil
	case "darwin":
		return filepath.Join(homePath, "Library", "Preferences", appName), nil
	case "linux":
		return filepath.Join(xdg.ConfigHome, appName), nil
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
