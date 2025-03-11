package cli

import (
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/model"
)

var (
	defaultWebHost       = "localhost"
	defaultWebPort       = model.DefaultWebPort
	defaultNamespace     = ""
	defaultLogLevel      = ""
	defaultLogSource     = "all"
	webHostFlag          = ""
	webPortFlag          = 0
	snapshotViewPortFlag = 0
	namespaceOverride    = ""
)

func readEnvDefaults() error {
	envPort := os.Getenv("TILT_PORT")
	if envPort != "" {
		port, err := strconv.Atoi(envPort)
		if err != nil {
			return errors.Wrap(err, "parsing env TILT_PORT")
		}
		defaultWebPort = port
	}

	envHost := os.Getenv("TILT_HOST")
	if envHost != "" {
		defaultWebHost = envHost
	}
	return nil
}

// Common flags used across multiple commands.

// s: address of the field to populate
func addTiltfileFlag(cmd *cobra.Command, s *string) {
	cmd.Flags().StringVarP(s, "file", "f", tiltfile.FileName, "Path to Tiltfile")
}

func addKubeContextFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&kubeContextOverride, "context", "", "Kubernetes context override. Equivalent to kubectl --context")
}

// For commands that talk to the web server.
func addConnectServerFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&webPortFlag, "port", defaultWebPort, "Port for the Tilt HTTP server. Only necessary if you started Tilt with --port. Overrides TILT_PORT env variable.")
	cmd.Flags().StringVar(&webHostFlag, "host", defaultWebHost, "Host for the Tilt HTTP server. Only necessary if you started Tilt with --host. Overrides TILT_HOST env variable.")
}

// For commands that start a web server.
func addStartServerFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&webPortFlag, "port", defaultWebPort, "Port for the Tilt HTTP server. Set to 0 to disable. Overrides TILT_PORT env variable.")
	cmd.Flags().StringVar(&webHostFlag, "host", defaultWebHost, "Host for the Tilt HTTP server and default host for any port-forwards. Set to 0.0.0.0 to listen on all interfaces. Overrides TILT_HOST env variable.")
}

// For commands that start a random snapshot view web server.
func addStartSnapshotViewServerFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&snapshotViewPortFlag, "port", 0, "Port for the HTTP server. Defaults to a random port.")
	cmd.Flags().StringVar(&webHostFlag, "host", defaultWebHost, "Host for the HTTP server and default host for any port-forwards. Set to 0.0.0.0 to listen on all interfaces. Overrides TILT_HOST env variable.")
}

func addDevServerFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&webDevPort, "webdev-port", DefaultWebDevPort, "Port for the Tilt Dev Webpack server. Only applies when using --web-mode=local")
	cmd.Flags().Var(&webModeFlag, "web-mode", "Values: local, prod. Controls whether to use prod assets or a local dev server. (If flag not specified: if Tilt was built from source, it will use a local asset server; otherwise, prod assets.)")
}

func addNamespaceFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&namespaceOverride, "namespace", defaultNamespace, "Default namespace for Kubernetes resources (overrides default namespace from active context in kubeconfig)")
}

func addLogFilterResourcesFlag(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&logResourcesFlag, "log-resource", nil, `Specify one or more resources to print logs for, e.g. "(Tiltfile)", "nginx", etc. If not specified, prints all resources.`)
}

func addLogFilterFlags(cmd *cobra.Command, prefix string) {
	cmd.Flags().StringVar(&logLevelFlag, prefix+"level", defaultLogLevel, `Specify a log level. One of "warn", "error"`)
	_ = cmd.RegisterFlagCompletionFunc(
		prefix+"level",
		func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var completions []string
			options := []string{"warn", "error"}
			for _, option := range options {
				if strings.Contains(option, strings.ToLower(toComplete)) {
					completions = append(completions, option)
				}
			}

			return completions, cobra.ShellCompDirectiveNoFileComp
		},
	)
	cmd.Flags().StringVar(&logSourceFlag, prefix+"source", defaultLogSource, `Specify a log source. One of "all", "build", "runtime"`)
	_ = cmd.RegisterFlagCompletionFunc(
		prefix+"source",
		func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var completions []string
			options := []string{
				hud.FilterSourceAll.String(),
				hud.FilterSourceBuild.String(),
				hud.FilterSourceRuntime.String(),
			}
			for _, option := range options {
				if strings.Contains(option, strings.ToLower(toComplete)) {
					completions = append(completions, option)
				}
			}

			return completions, cobra.ShellCompDirectiveNoFileComp
		},
	)
}

var kubeContextOverride string

func ProvideKubeContextOverride() k8s.KubeContextOverride {
	return k8s.KubeContextOverride(kubeContextOverride)
}

func ProvideNamespaceOverride() k8s.NamespaceOverride {
	return k8s.NamespaceOverride(namespaceOverride)
}
