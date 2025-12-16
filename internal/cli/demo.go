package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/tilt-dev/go-get"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/cli/demo"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const demoResourcesPrefix = "tilt-demo-"
const sampleProjPackage = "github.com/tilt-dev/tilt-avatars"

type demoCmd struct {
	// legacy disables the web UI (this is only used for integration tests)
	legacy bool
	// teardown will clean up any leftover `tilt demo` clusters and exit
	teardown bool
	// tmpdir for cloned `tilt-avatars` resources
	tmpdir string
	// skipCreateCluster uses default kubeconfig context instead of creating
	// an ephemeral cluster
	skipCreateCluster bool
	// projPackage is the `go get` style URL for the demo project
	projPackage string
	// tiltfilePath is a path to a Tiltfile to launch instead of cloning and
	// running the `tilt-avatars` project
	tiltfilePath string
}

func (c *demoCmd) name() model.TiltSubcommand { return "demo" }

func (c *demoCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo [flags]",
		Short: "Creates a local, temporary Kubernetes cluster and runs a Tilt sample project",
		Long: fmt.Sprintf(`Test out Tilt using an isolated, ephemeral local Kubernetes setup.

Tilt will create a temporary, local Kubernetes development cluster running in Docker.
The cluster will be removed when Tilt is exited with Ctrl-C.

A sample project (%s) will be cloned locally to a temporary directory using Git and launched.
`, sampleProjPackage),
	}

	cmd.Flags().BoolVarP(&c.teardown, "teardown", "", false,
		"Removes any leftover tilt-demo Kubernetes clusters and exits")

	// --legacy flag only exists for integration tests to disable web console
	cmd.Flags().BoolVar(&c.legacy, "legacy", false, "If true, tilt will open in legacy HUD mode.")
	cmd.Flags().Lookup("legacy").Hidden = true

	// --tmpdir exists so that integration tests can inspect the output / use the Tiltfile
	cmd.Flags().StringVarP(&c.tmpdir, "tmpdir", "", "",
		"Temporary directory to clone sample project to")
	cmd.Flags().Lookup("tmpdir").Hidden = true

	cmd.Flags().BoolVar(&c.skipCreateCluster, "no-cluster", false,
		"Skip ephemeral cluster creation (requires local K8s cluster to already be configured)")

	cmd.Flags().StringVarP(&c.projPackage, "repo", "r", sampleProjPackage,
		"Path to custom repo to use instead of Tiltfile")

	// we don't use the `addTiltfileFlag()` because the default here should be empty
	cmd.Flags().StringVarP(&c.tiltfilePath, "file", "f", "",
		"Path to custom Tiltfile to use instead of sample project")

	addStartServerFlags(cmd)
	addDevServerFlags(cmd)

	return cmd
}

func (c *demoCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.demo", map[string]string{})
	defer a.Flush(time.Second)

	client, err := wireDockerLocalClient(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to init Docker client")
	}
	k3dCli := demo.NewK3dClient(client)

	if c.teardown {
		return c.cleanupClusters(ctx, k3dCli)
	}

	if c.projPackage != sampleProjPackage && c.tiltfilePath != "" {
		return fmt.Errorf("cannot specify both a custom repo and Tiltfile path")
	}

	//
	// 0. Prepare environment
	//
	logger.Get(ctx).Infof("\nHang tight while Tilt prepares your demo environment!")
	c.tmpdir, err = os.MkdirTemp(c.tmpdir, demoResourcesPrefix)
	if err != nil {
		return fmt.Errorf("could not create temporary directory: %v", err)
	}

	if !c.skipCreateCluster {
		err = client.CheckConnected()
		if err != nil {
			return fmt.Errorf("tilt demo requires Docker to be installed and running: %v", err)
		}
		if !isLocalDockerHost(client.Env().DaemonHost()) {
			// properly supporting remote Docker connections is very tricky - either:
			//
			// the remote host will need more ports accessible (for K8s API + registry API) and we have to ensure
			// that everything both listens on the public interface and references it in configs
			// (such as "local-registry-hosting" ConfigMap)
			// 	OR
			// we need to tunnel everything (perhaps using Docker - this is the approach ctlptl takes!)
			//
			// for now, it's not supported as it's a pretty advanced setup to begin with, so we're not really targeting
			// it with the `tilt demo` functionality
			return fmt.Errorf("tilt demo requires a local Docker daemon to create a temporary Kubernetes cluster (current Docker host: %s)", client.Env().DaemonHost())
		}

		//
		// 1. Create a cluster that will be torn down in the background on exit (Ctrl-C)
		//
		clusterName := filepath.Base(c.tmpdir)
		logger.Get(ctx).Infof("\tCreating %q local Kubernetes cluster...", clusterName)
		if err := k3dCli.CreateCluster(ctx, clusterName); err != nil {
			return fmt.Errorf("failed to create Kubernetes cluster: %v", err)
		}
		defer func() {
			// N.B. use background context because the main context has already been canceled due to Ctrl-C
			// 	but also don't block on execution (just fire request to Docker API and forget) because at this
			// 	point we have < 2 secs before the signal handler forcibly exits the process
			ctx := logger.WithLogger(context.Background(), logger.Get(ctx))
			logger.Get(ctx).Infof("\nDeleting %q local Kubernetes cluster...", clusterName)
			if err = k3dCli.DeleteCluster(ctx, clusterName, false); err != nil {
				logger.Get(ctx).Warnf("\tFailed to delete cluster %q: %v", clusterName, err)
			}
		}()

		//
		// 2. Use the new cluster's kubeconfig for this Tilt process
		//
		if kubeconfig, err := k3dCli.GenerateKubeconfig(ctx, clusterName); err != nil {
			return fmt.Errorf("failed to generate kubeconfig: %v", err)
		} else {
			// Replace "host.docker.internal" with "localhost" in the kubeconfig for docker desktop.
			kubeconfig = bytes.ReplaceAll(kubeconfig,
				[]byte("host.docker.internal"), []byte("localhost"))

			kubeconfigPath := filepath.Join(c.tmpdir, "kubeconfig")
			if err := os.WriteFile(kubeconfigPath, kubeconfig, 0666); err != nil {
				return fmt.Errorf("failed to write kubeconfig file: %v", err)
			}
			err = os.Setenv("KUBECONFIG", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to set KUBECONFIG env var: %v", err)
			}
		}
	}

	//
	// 3. Download the sample project to a tmpdir
	//
	var projPath string
	if c.tiltfilePath == "" {
		logger.Get(ctx).Infof("\tFetching %q project...", c.projPackage)
		dlr := get.NewDownloader(c.tmpdir)
		projPath, err = dlr.Download(c.projPackage)
		if err != nil {
			return fmt.Errorf("failed to download sample project: %v", err)
		}
		c.tiltfilePath = filepath.Join(projPath, "Tiltfile")
	}

	logger.Get(ctx).Infof("\tDone!")
	if projPath != "" {
		logger.Get(ctx).Infof(
			`
-----------------------------------------------------
Open the project directory in your preferred editor:
  %s
-----------------------------------------------------
`, color.BlueString("%s", projPath))
	}

	//
	// 4. Launch the `tilt up` command with the sample project
	// 		(it will implicitly use our kubeconfig)
	//
	up := upCmd{
		fileName: c.tiltfilePath,
		legacy:   c.legacy,
		stream:   false,
	}
	return up.run(ctx, args)
}

func (c *demoCmd) cleanupClusters(ctx context.Context, k3dCli *demo.K3dClient) error {
	clusterNames, err := k3dCli.ListClusters(ctx)
	if err != nil {
		return err
	}

	failed := false
	for _, clusterName := range clusterNames {
		if strings.HasPrefix(clusterName, demoResourcesPrefix) {
			logger.Get(ctx).Infof("Removing cluster %q...", clusterName)
			if err := k3dCli.DeleteCluster(ctx, clusterName, true); err != nil {
				failed = true
				logger.Get(ctx).Errorf("Failed to remove %q cluster: %v", clusterName, err)
			}
		}
	}
	if failed {
		return errors.New("could not remove one or more tilt-demo K8s clusters")
	}
	return nil
}

// TODO(milas): this is copy-pasted from ctlptl, use it from a common place
func isLocalDockerHost(dockerHost string) bool {
	return dockerHost == "" ||

		// Check all the "standard" docker localhosts.
		// https://github.com/docker/cli/blob/a32cd16160f1b41c1c4ae7bee4dac929d1484e59/opts/hosts.go#L22
		strings.HasPrefix(dockerHost, "tcp://localhost:") ||
		strings.HasPrefix(dockerHost, "tcp://127.0.0.1:") ||

		// https://github.com/moby/moby/blob/master/client/client_windows.go#L4
		strings.HasPrefix(dockerHost, "npipe:") ||

		// https://github.com/moby/moby/blob/master/client/client_unix.go#L6
		strings.HasPrefix(dockerHost, "unix:")
}
