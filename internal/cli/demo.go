package cli

import (
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

const demoClusterPrefix = "tilt-demo-"
const sampleProjPackage = "github.com/tilt-dev/tilt-avatars"

type demoCmd struct {
	tiltfilePath string
	teardown     bool
	hud          bool
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

	// --hud flag only exists for integration tests to disable web console
	cmd.Flags().BoolVar(&c.hud, "hud", true, "If true, tilt will open in HUD mode.")
	cmd.Flags().Lookup("hud").Hidden = true

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

	client, err := wireDockerClusterClient(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to init Docker client")
	}
	k3dCli := demo.NewK3dClient(client)

	//
	// 0. Prepare environment
	//
	err = client.CheckConnected()
	if err != nil {
		return fmt.Errorf("tilt demo requires Docker to be installed and running: %v", err)
	}
	if client.Env().Host != "" && !strings.HasPrefix(client.Env().Host, "unix://") {
		// to properly support remote Docker hosts, we'd need to:
		// 	* manually create the registry with `k3d registry create tilt-demo-xxxx --port $HOST:random
		// 		* query the registry back to find the port number
		// 	* use the registry when creating cluster `k3d cluster create tilt-demo-xxx --registry use tilt-demo-xxxx:$PORT`
		// 	* parse the kubeconfig and rewrite the cluster host to use $HOST instead of 0.0.0.0
		return fmt.Errorf("tilt demo requires a local Docker daemon (current Docker host: %s)", client.Env().Host)
	}

	if c.teardown {
		return c.cleanupClusters(ctx, k3dCli)
	}

	logger.Get(ctx).Infof("\nHang tight while Tilt prepares your demo environment!")

	//
	// 1. Create a cluster that will be torn down in the background on exit (Ctrl-C)
	//
	tmpdir, err := os.MkdirTemp("", demoClusterPrefix)
	if err != nil {
		return fmt.Errorf("could not create temporary directory: %v", err)
	}
	clusterName := filepath.Base(tmpdir)
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
		kubeconfigPath := filepath.Join(tmpdir, "kubeconfig")
		if err := os.WriteFile(kubeconfigPath, kubeconfig, 0666); err != nil {
			return fmt.Errorf("failed to write kubeconfig file: %v", err)
		}
		err = os.Setenv("KUBECONFIG", kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to set KUBECONFIG env var: %v", err)
		}
	}

	//
	// 3. Download the sample project to a tmpdir
	//
	var projPath string
	if c.tiltfilePath == "" {
		logger.Get(ctx).Infof("\tFetching %q project...", sampleProjPackage)
		dlr := get.NewDownloader(tmpdir)
		projPath, err = dlr.Download(sampleProjPackage)
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
		hud:      c.hud,
		legacy:   false,
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
		if strings.HasPrefix(clusterName, demoClusterPrefix) {
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
