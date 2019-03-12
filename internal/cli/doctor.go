package cli

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
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
	analyticsService.Incr("cmd.doctor", map[string]string{})
	defer analyticsService.Flush(time.Second)

	fmt.Printf("Tilt: %s\n", buildStamp())
	fmt.Printf("System: %s-%s\n", runtime.GOOS, runtime.GOARCH)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	fmt.Println("---")
	fmt.Println("Docker")

	dockerEnv, err := wireDockerEnv(ctx)
	if err != nil {
		printField("Host", nil, err)
	} else {
		host := dockerEnv.Host
		if host == "" {
			host = "[default]"
		}
		printField("Host", host, err)
	}

	dockerVersion, err := wireDockerVersion(ctx)
	if err != nil {
		printField("Version", "", err)
	} else {
		printField("Version", dockerVersion.APIVersion, err)
	}

	fmt.Println("---")
	fmt.Println("Kubernetes")

	env, err := wireEnv(ctx)
	printField("Env", env, err)

	kContext, err := wireKubeContext(ctx)
	printField("Context", kContext, err)

	ns, err := wireNamespace(ctx)
	printField("Namespace", ns, err)

	runtime, err := wireRuntime(ctx)
	printField("Container Runtime", runtime, err)

	kVersion, err := wireK8sVersion(ctx)
	printField("Version", kVersion, err)

	fmt.Println("---")
	fmt.Println("Thanks for seeing the Tilt Doctor!")
	fmt.Println("Please send this info along when filing bug reports. 💗")

	return nil
}

func printField(name string, v interface{}, err error) {
	if err != nil {
		fmt.Printf("- %s: Error: %v\n", name, err)
	} else {
		fmt.Printf("- %s: %s\n", name, v)
	}
}
