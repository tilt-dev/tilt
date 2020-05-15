package cli

import (
	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/tiltfile"
)

// Common flags used across multiple commands.

// s: address of the field to populate
func addTiltfileFlag(cmd *cobra.Command, s *string) {
	cmd.Flags().StringVarP(s, "file", "f", tiltfile.FileName, "Path to Tiltfile")
}

// For commands that talk to the web server.
func addConnectServerFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server. Only necessary if you started Tilt with --port.")
	cmd.Flags().StringVar(&webHost, "host", DefaultWebHost, "Host for the Tilt HTTP server. Only necessary if you started Tilt with --host.")
}

// For commands that start a web server.
func addStartServerFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server. Set to 0 to disable.")
	cmd.Flags().StringVar(&webHost, "host", DefaultWebHost, "Host for the Tilt HTTP server and default host for any port-forwards. Set to 0.0.0.0 to listen on all interfaces.")
}

func addDevServerFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&webDevPort, "webdev-port", DefaultWebDevPort, "Port for the Tilt Dev Webpack server. Only applies when using --web-mode=local")
	cmd.Flags().Var(&webModeFlag, "web-mode", "Values: local, prod. Controls whether to use prod assets or a local dev server")
}
