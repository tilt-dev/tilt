package xdg

import (
	"fmt"
	"os"

	"github.com/tilt-dev/tilt/pkg/model"
)

// Identifies the Tilt process that owns a piece of on-disk state, so that
// concurrent Tilt processes never write to, or clean up, each other's.
//
// Identified by the web-port (for long-running commands that hold a port),
// or by the pid (for one-shot commands that don't hold a port).
type CLIWorkspaceID string

func ProvideCLIWorkspaceID(subcommand model.TiltSubcommand, port model.WebPort) CLIWorkspaceID {
	if subcommand.BindsWebPort() && port != 0 {
		return CLIWorkspaceID(model.DefaultAPIServerName(port))
	}
	return CLIWorkspaceID(fmt.Sprintf("tilt-pid-%d", os.Getpid()))
}
