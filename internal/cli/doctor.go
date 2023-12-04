package cli

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type doctorCmd struct {
}

func (c *doctorCmd) name() model.TiltSubcommand { return "doctor" }

func (c *doctorCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Print diagnostic information about the Tilt environment, for filing bug reports",
	}
	addKubeContextFlag(cmd)
	return cmd
}

func (c *doctorCmd) run(ctx context.Context, args []string) error {
	analytics.Get(ctx).Incr("cmd.doctor", map[string]string{})
	defer analytics.Get(ctx).Flush(time.Second)

	fmt.Printf("Tilt: %s\n", buildStamp())
	fmt.Printf("System: %s-%s\n", runtime.GOOS, runtime.GOARCH)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var localDocker, clusterDocker docker.Client
	var localDockerErr, clusterDockerErr error
	multipleClients := false
	client, err := wireDockerCompositeClient(ctx)
	if err == nil {
		localDocker = client.DefaultLocalClient()
		clusterDocker = client.DefaultClusterClient()
		multipleClients = client.HasMultipleClients()
	} else { // Figure out which client(s) had errors so we can show them
		localDocker, localDockerErr = wireDockerLocalClient(ctx)
		clusterDocker, clusterDockerErr = wireDockerClusterClient(ctx)
		multipleClients = (localDockerErr != nil) != (clusterDockerErr != nil)
	}

	fmt.Println("---")
	if multipleClients {
		fmt.Println("Docker (cluster)")
	} else {
		fmt.Println("Docker")
	}

	if clusterDockerErr != nil {
		printField("Host", nil, clusterDockerErr)
	} else {
		dockerEnv := clusterDocker.Env()
		host := dockerEnv.DaemonHost()
		if host == "" {
			host = "[default]"
		}
		printField("Host", host, nil)

		version := clusterDocker.ServerVersion()
		printField("Server Version", version.Version, nil)
		printField("API Version", version.APIVersion, nil)

		builderVersion := clusterDocker.BuilderVersion()
		printField("Builder", builderVersion, nil)
	}

	if multipleClients {
		fmt.Println("---")
		fmt.Println("Docker (local)")

		if localDockerErr != nil {
			printField("Host", nil, localDockerErr)
		} else {
			dockerEnv := localDocker.Env()
			host := dockerEnv.DaemonHost()
			if host == "" {
				host = "[default]"
			}
			printField("Host", host, nil)

			version := localDocker.ServerVersion()
			printField("Server Version", version.Version, nil)
			printField("Version", version.APIVersion, nil)

			builderVersion := localDocker.BuilderVersion()
			printField("Builder", builderVersion, nil)
		}
	}

	// in theory, the env shouldn't matter since we're just calling the version subcommand,
	// but to be safe, we'll try to use the actual local env if available
	composeEnv := docker.LocalEnv{}
	if localDockerErr == nil {
		composeEnv = docker.LocalEnv(localDocker.Env())
	}
	dcCli := dockercompose.NewDockerComposeClient(composeEnv)
	// errors getting the version aren't generally useful; in many cases it'll just mean that
	// the command couldn't exec since Docker Compose isn't installed, for example, so they
	// are just ignored and the field skipped
	if composeVersion, composeBuild, err := dcCli.Version(ctx); err == nil {
		composeField := composeVersion
		if composeBuild != "" {
			composeField += fmt.Sprintf(" (build %s)", composeBuild)
		}
		printField("Compose Version", composeField, nil)
	}

	fmt.Println("---")
	fmt.Println("Kubernetes")

	env, err := wireEnv(ctx)
	printField("Env", env, err)

	kContext, err := wireKubeContext(ctx)
	printField("Context", kContext, err)
	clusterName, err := wireClusterName(ctx)
	if clusterName == "" {
		clusterName = "Unknown"
	}
	printField("Cluster Name", clusterName, err)

	ns, err := wireNamespace(ctx)
	printField("Namespace", ns, err)

	runtime, err := containerRuntime(ctx)
	printField("Container Runtime", runtime, err)

	kVersion, err := wireK8sVersion(ctx)
	printField("Version", kVersion, err)

	registryDisplay, err := clusterLocalRegistryDisplay(ctx)
	printField("Cluster Local Registry", registryDisplay, err)

	fmt.Println("---")
	fmt.Println("Thanks for seeing the Tilt Doctor!")
	fmt.Println("Please send the info above when filing bug reports. 💗")

	fmt.Println("")
	fmt.Println("The info below helps us understand how you're using Tilt so we can improve,")
	fmt.Println("but is not required to ask for help.")

	fmt.Println("---")
	fmt.Println("Analytics Settings")
	fmt.Println("--> (These results reflect your personal opt in/out status and may be overridden by an `analytics_settings` call in your Tiltfile)")

	a := analytics.Get(ctx)
	opt := a.UserOpt()
	fmt.Printf("- User Mode: %s\n", opt)

	fmt.Printf("- Machine: %s\n", a.MachineHash())
	fmt.Printf("- Repo: %s\n", a.GitRepoHash())

	return nil
}

func containerRuntime(ctx context.Context) (container.Runtime, error) {
	kClient, err := wireK8sClient(ctx)
	if err != nil {
		return "", err
	}
	return kClient.ContainerRuntime(ctx), nil
}

func clusterLocalRegistryDisplay(ctx context.Context) (string, error) {
	kClient, err := wireK8sClient(ctx)
	if err != nil {
		return "", err
	}

	// blackhole any warnings
	newCtx := logger.WithLogger(ctx, logger.NewDeferredLogger(ctx))
	registry := kClient.LocalRegistry(newCtx)
	if container.IsEmptyRegistry(registry) {
		return "none", nil
	}
	return fmt.Sprintf("%+v", registry), nil
}

func printField(name string, v interface{}, err error) {
	if err != nil {
		fmt.Printf("- %s: Error: %v\n", name, err)
	} else {
		fmt.Printf("- %s: %s\n", name, v)
	}
}
