package cli

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	wmanalytics "github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/analytics"
)

type doctorCmd struct {
}

func (c *doctorCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Print diagnostic information about the Tilt environment, for filing bug reports",
	}
	return cmd
}

func (c *doctorCmd) run(ctx context.Context, args []string) error {
	analytics.Get(ctx).Incr("cmd.doctor", map[string]string{})
	defer analytics.Get(ctx).Flush(time.Second)

	fmt.Printf("Tilt: %s\n", buildStamp())
	fmt.Printf("System: %s-%s\n", runtime.GOOS, runtime.GOARCH)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	clusterDocker, clusterDockerErr := wireDockerClusterClient(ctx)
	if clusterDockerErr == nil {
		clusterDockerErr = clusterDocker.CheckConnected()
	}

	localDocker, localDockerErr := wireDockerLocalClient(ctx)
	if localDockerErr == nil {
		localDockerErr = localDocker.CheckConnected()
	}

	isLocalDockerErr := localDockerErr != nil
	isClusterDockerErr := clusterDockerErr != nil
	twoDockerClients := (isLocalDockerErr != isClusterDockerErr) ||
		(!isLocalDockerErr && !isClusterDockerErr && localDocker.Env().Host != clusterDocker.Env().Host)

	fmt.Println("---")
	if twoDockerClients {
		fmt.Println("Docker (cluster)")
	} else {
		fmt.Println("Docker")
	}

	if clusterDockerErr != nil {
		printField("Host", nil, clusterDockerErr)
	} else {
		dockerEnv := clusterDocker.Env()
		host := dockerEnv.Host
		if host == "" {
			host = "[default]"
		}
		printField("Host", host, nil)

		version := clusterDocker.ServerVersion()
		printField("Version", version.APIVersion, nil)

		builderVersion := clusterDocker.BuilderVersion()
		printField("Builder", builderVersion, nil)
	}

	if twoDockerClients {
		fmt.Println("---")
		fmt.Println("Docker (local)")

		if localDockerErr != nil {
			printField("Host", nil, localDockerErr)
		} else {
			dockerEnv := localDocker.Env()
			host := dockerEnv.Host
			if host == "" {
				host = "[default]"
			}
			printField("Host", host, nil)

			version := localDocker.ServerVersion()
			printField("Version", version.APIVersion, nil)

			builderVersion := localDocker.BuilderVersion()
			printField("Builder", builderVersion, nil)
		}
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

	runtime, err := wireRuntime(ctx)
	printField("Container Runtime", runtime, err)

	kVersion, err := wireK8sVersion(ctx)
	printField("Version", kVersion, err)

	fmt.Println("---")
	fmt.Println("Thanks for seeing the Tilt Doctor!")
	fmt.Println("Please send the info above when filing bug reports. 💗")

	fmt.Println("")
	fmt.Println("The info below helps us understand how you're using Tilt so we can improve,")
	fmt.Println("but is not required to ask for help.")

	fmt.Println("---")
	fmt.Println("Analytics Settings")

	a := analytics.Get(ctx)
	opt := a.UserOpt()
	fmt.Printf("- User Mode: %s\n", opt)

	machineHash := "[redacted]"
	gitRepoHash := "[redacted]"
	if opt == wmanalytics.OptIn {
		machineHash = a.MachineHash()
		gitRepoHash = a.GitRepoHash()
	}

	fmt.Printf("- Machine: %s\n", machineHash)
	fmt.Printf("- Repo: %s\n", gitRepoHash)

	return nil
}

func printField(name string, v interface{}, err error) {
	if err != nil {
		fmt.Printf("- %s: Error: %v\n", name, err)
	} else {
		fmt.Printf("- %s: %s\n", name, v)
	}
}
