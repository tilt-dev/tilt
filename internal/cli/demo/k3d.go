package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/mount"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/pkg/logger"
)

const defaultK3dImage = "docker.io/rancher/k3d:v4.4.7"

type cluster struct {
	Name string `json:"name"`
}

type K3dClient struct {
	cli      docker.Client
	k3dImage reference.Named

	ensurePulled sync.Once
}

func NewK3dClient(cli docker.Client) *K3dClient {
	ref, err := reference.ParseNamed(defaultK3dImage)
	if err != nil {
		panic(fmt.Errorf("invalid image ref %q: %v", defaultK3dImage, err))
	}

	return &K3dClient{
		cli:      cli,
		k3dImage: ref,
	}
}

func (k *K3dClient) ListClusters(ctx context.Context) ([]string, error) {
	cmd := []string{"cluster", "list", "-ojson"}
	var clusterListJson bytes.Buffer
	stderr := logger.Get(ctx).Writer(logger.WarnLvl)
	if err := k.command(ctx, cmd, &clusterListJson, stderr, true); err != nil {
		return nil, err
	}

	var clusters []cluster
	if err := json.Unmarshal(clusterListJson.Bytes(), &clusters); err != nil {
		return nil, fmt.Errorf("invalid JSON output from cluster list: %v", err)
	}

	clusterNames := make([]string, len(clusters))
	for i := range clusters {
		clusterNames[i] = clusters[i].Name
	}
	return clusterNames, nil
}

func (k *K3dClient) CreateCluster(ctx context.Context, clusterName string) error {
	cmd := []string{
		"cluster",
		"create", clusterName,
		"--registry-create",
		"--kubeconfig-switch-context",
		"--kubeconfig-update-default",
		"--no-hostip",
		"--no-image-volume",
		"--no-lb",
		"--label", fmt.Sprintf("%s@%s", docker.BuiltByTiltLabelStr, "server[0]"),
	}
	stdout := logger.Get(ctx).Writer(logger.DebugLvl)
	stderr := logger.Get(ctx).Writer(logger.WarnLvl)
	if err := k.command(ctx, cmd, stdout, stderr, true); err != nil {
		return err
	}
	return nil
}

func (k *K3dClient) DeleteCluster(ctx context.Context, clusterName string, wait bool) error {
	cmd := []string{
		"cluster",
		"delete", clusterName,
	}
	var stdout, stderr io.Writer
	if wait {
		log := logger.Get(ctx)
		stdout = log.Writer(logger.DebugLvl)
		stderr = logger.NewFuncLogger(log.SupportsColor(), log.Level(),
			func(level logger.Level, fields logger.Fields, b []byte) error {
				// there's no kubeconfig in the container so k3d will emit confusing warnings
				// note: no kubeconfig cleanup is necessary since k3d's execution is isolated
				// 	via docker, so is never touching the host filesystem, but it's a weird
				// 	use case so k3d doesn't have a flag to disable kubeconfig cleanup on delete
				if bytes.Contains(b, []byte("Failed to remove cluster details")) ||
					bytes.Contains(b, []byte("no such file or directory")) {
					return nil
				}
				log.Write(logger.WarnLvl, b)
				return nil
			}).Writer(logger.WarnLvl)
	}
	if err := k.command(ctx, cmd, stdout, stderr, wait); err != nil {
		return err
	}
	return nil
}

func (k *K3dClient) GenerateKubeconfig(ctx context.Context, clusterName string) ([]byte, error) {
	var kubeconfigBuf bytes.Buffer
	stderr := logger.Get(ctx).Writer(logger.WarnLvl)
	err := k.command(ctx, []string{"kubeconfig", "get", clusterName}, &kubeconfigBuf, stderr, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %v", err)
	}
	return kubeconfigBuf.Bytes(), nil
}

func (k *K3dClient) command(ctx context.Context, cmd []string, stdout io.Writer, stderr io.Writer, wait bool) error {
	// lazily pull the image the first time a command is run to avoid network-induced latency checking for an
	// up-to-date image on each command
	k.ensurePulled.Do(func() {
		ref, err := k.cli.ImagePull(ctx, k.k3dImage)
		if err != nil {
			logger.Get(ctx).Errorf("failed to pull %q image: %v", k.k3dImage, err)
		} else {
			k.k3dImage = ref
		}
	})

	runConfig := docker.RunConfig{
		Pull:   false,
		Stdout: stdout,
		Stderr: stderr,
		Image:  k.k3dImage,
		Cmd:    cmd,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/var/run/docker.sock",
				Target: "/var/run/docker.sock",
			},
		},
	}
	runResult, err := k.cli.Run(ctx, runConfig)
	if err != nil {
		return fmt.Errorf("failed to run `k3d %s`: %v", strings.Join(cmd, " "), err)
	}
	if wait {
		defer func() {
			if err := runResult.Close(); err != nil {
				logger.Get(ctx).Debugf("Failed to clean up container %q: %v", runResult.ContainerID, err)
			}
		}()
		status, err := runResult.Wait()
		if err != nil {
			return err
		}
		if status != 0 {
			return fmt.Errorf("k3d exited with code: %d", status)
		}
	}
	return nil
}
