package main

import (
	"flag"
	"log"
	"os"

	"github.com/windmilleng/tilt/internal/cli"
	"k8s.io/klog"
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

// HACK(nick): The Kubernetes libs we use sometimes use klog to log things to
// os.Stderr. There are no API hooks to configure this. And printing to Stderr
// scrambles the HUD tty.
//
// Fortunately, klog does initialize itself from flags, so we can backdoor
// configure it by setting our own flags. Don't do this at home!
func disableGlog() {
	var tmpFlagSet = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(tmpFlagSet)
	err := tmpFlagSet.Parse([]string{
		"--stderrthreshold", "FATAL",
	})
	if err != nil {
		log.Fatal(err)
	}
}
