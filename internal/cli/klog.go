package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"k8s.io/klog"
)

var klogLevel = 0

// everything on google indicates this warning is useless and should be ignored
func isResourceVersionTooOldMessage(s string) bool {
	return strings.Contains(s, "reflector.go") && strings.Contains(s, "The resourceVersion for the provided watch is too old.")
}

// unfortunately, k8s logs spurious and annoying messages via glog warnings, and there's no good way to control that
// via command line flags, so we're just gonna apply string filtering!
// `filteredWriter` creates a new `io.Writer` that passes through to `w` only those lines for which
// 	 `filterFunc` returns false
func filteredWriter(w io.Writer, filterFunc func(s string) bool) io.Writer {
	r, fw := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Split(bufio.ScanLines)
		for {
			if !scanner.Scan() {
				break
			}

			s := scanner.Text()
			if !filterFunc(s) {
				_, err := w.Write([]byte(s + "\n"))
				if err != nil {
					_ = r.CloseWithError(err)
				}
			}
		}
	}()

	return fw
}

// HACK(nick): The Kubernetes libs we use sometimes use klog to log things to
// os.Stderr. There are no API hooks to configure this. And printing to Stderr
// scrambles the HUD tty.
//
// Fortunately, klog does initialize itself from flags, so we can backdoor
// configure it by setting our own flags. Don't do this at home!
func initKlog(w io.Writer) {
	var tmpFlagSet = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(tmpFlagSet)

	if klogLevel == 0 {
		w = filteredWriter(w, isResourceVersionTooOldMessage)
	}
	klog.SetOutput(w)

	flags := []string{
		"--stderrthreshold", "FATAL",
	}

	if klogLevel > 0 {
		flags = append(flags, "-v", fmt.Sprintf("%d", klogLevel))
	}

	err := tmpFlagSet.Parse(flags)
	if err != nil {
		log.Fatal(err)
	}
}
