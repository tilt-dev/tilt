package cli

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"

	"github.com/tilt-dev/tilt/internal/filteredwriter"

	"k8s.io/klog/v2"
)

// https://kubernetes.io/docs/reference/kubectl/cheatsheet/#kubectl-output-verbosity-and-debugging
var klogLevel = 0

// everything on google indicates this warning is useless and should be ignored
// https://github.com/kubernetes/kubernetes/issues/22024
var isResourceVersionTooOldRegexp = regexp.MustCompile("reflector.go.*watch of.*ended with: too old resource version")

// isGroupVersionEmptyRegexp matches errors that are logged deep within K8s when
// populating the cache of resource types for a group-version if the group-version
// has no resource types registered. This is an uncommon but valid scenario that
// most commonly occurs with prometheus-adapter and the `external.metrics.k8s.io/v1beta1`
// group if no external metrics are actually registered (but the group will still
// exist). The K8s code returns an error instead of treating it as empty, which gets
// logged at an error level so will show up in Tilt logs and leads to confusion,
// particularly on `tilt down` where they show up mixed in the CLI output.
// https://github.com/kubernetes/kubernetes/blob/a4e5239bdc3d85f1f90c7811b03dc153d5121ffe/staging/src/k8s.io/client-go/discovery/cached/memory/memcache.go#L212-L221
// https://github.com/kubernetes/kubernetes/issues/92480
var isGroupVersionEmptyRegexp = regexp.MustCompile("couldn't get resource list for.*Got empty response")

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
		w = filteredwriter.New(w, filterMux(
			isResourceVersionTooOldRegexp.MatchString,
			isGroupVersionEmptyRegexp.MatchString,
		))
	}
	klog.SetOutput(w)

	flags := []string{
		"--stderrthreshold=FATAL",
		"--logtostderr=false",
	}

	if klogLevel > 0 {
		flags = append(flags, "-v", fmt.Sprintf("%d", klogLevel))
	}

	err := tmpFlagSet.Parse(flags)
	if err != nil {
		log.Fatal(err)
	}
}

// filterMux combines multiple filter functions, returning true if any match.
func filterMux(filterFuncs ...func(s string) bool) func(s string) bool {
	return func(s string) bool {
		for _, fn := range filterFuncs {
			if fn(s) {
				return true
			}
		}
		return false
	}
}
