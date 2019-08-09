package cli

import (
	"flag"
	"fmt"
	"log"
	"os"

	"k8s.io/klog"
)

var klogLevel = 0

// HACK(nick): The Kubernetes libs we use sometimes use klog to log things to
// os.Stderr. There are no API hooks to configure this. And printing to Stderr
// scrambles the HUD tty.
//
// Fortunately, klog does initialize itself from flags, so we can backdoor
// configure it by setting our own flags. Don't do this at home!
func initKlog() {
	var tmpFlagSet = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(tmpFlagSet)

	flags := []string{
		"--stderrthreshold", "FATAL",
		// k8s' reflector.go currently has only spurious warnings
		// by default, set it to v0 so that they are not shown
		fmt.Sprintf("--vmodule=reflector=%d", klogLevel),
	}

	if klogLevel > 0 {
		flags = append(flags, "-v", fmt.Sprintf("%d", klogLevel))
	}

	err := tmpFlagSet.Parse(flags)
	if err != nil {
		log.Fatal(err)
	}
}
