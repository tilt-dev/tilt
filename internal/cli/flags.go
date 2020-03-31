package cli

import "github.com/spf13/cobra"

// Common flags used across multiple commands.

func addWebPortFlag(cmd *cobra.Command) {
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server. Set to 0 to disable.")
}

func addWebServerFlags(cmd *cobra.Command) {
	addWebPortFlag(cmd)
	cmd.Flags().StringVar(&webHost, "host", DefaultWebHost, "Host for the Tilt HTTP server and default host for any port-forwards. Set to 0.0.0.0 to listen on all interfaces.")
	cmd.Flags().IntVar(&webDevPort, "webdev-port", DefaultWebDevPort, "Port for the Tilt Dev Webpack server. Only applies when using --web-mode=local")
}
