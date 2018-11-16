package main

import (
	"flag"
	"log"

	"github.com/windmilleng/tilt/internal/cli"
)

// Magic variables set by goreleaser
var version string
var commit string
var date string

func main() {
	disableGlog()

	cli.SetBuildInfo(cli.BuildInfo{
		Version: version,
		Date:    date,
	})
	cli.Execute()
}

// HACK(nick): The Kubernetes libs we use sometimes use glog to log things to
// os.Stderr. There are no API hooks to configure this. And printing to Stderr
// scrambles the HUD tty.
//
// Fortunately, glog does initialize itself from flags, so we can backdoor
// configure it by setting our own flags. Don't do this at home! This works
// OK because we use Cobra for flags.
func disableGlog() {
	err := flag.CommandLine.Parse([]string{
		"--stderrthreshold", "FATAL",
	})
	if err != nil {
		log.Fatal(err)
	}
}
