package controllers

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// https://kubernetes.io/docs/reference/kubectl/cheatsheet/#kubectl-output-verbosity-and-debugging
var klogLevel = 0

func AddKlogFlags(flags *pflag.FlagSet) {
	flags.IntVar(&klogLevel, "klog", 0, "Enable Kubernetes API logging. Uses klog v-levels (0-4 are debug logs, 5-9 are tracing logs)")
}

// HACK(nick): The Kubernetes libs we use sometimes use klog to log things to
// os.Stderr. There are no API hooks to configure this. And printing to Stderr
// scrambles the HUD tty.
//
// Fortunately, klog does initialize itself from flags, so we can backdoor
// configure it by setting our own flags. Don't do this at home!
func InitKlog(w io.Writer) {
	var tmpFlagSet = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(tmpFlagSet)
	MaybeSetKlogOutput(w)

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

// We've historically had a lot of problems with bad klog output. For example:
//
// everything on google indicates this warning is useless and should be ignored
// https://github.com/kubernetes/kubernetes/issues/22024
//
// errors that are logged deep within K8s when populating the cache of resource
// types for a group-version if the group-version has no resource types
// registered. This is an uncommon but valid scenario that most commonly occurs
// with prometheus-adapter and the `external.metrics.k8s.io/v1beta1` group if no
// external metrics are actually registered (but the group will still
// exist). The K8s code returns an error instead of treating it as empty, which
// gets logged at an error level so will show up in Tilt logs and leads to
// confusion, particularly on `tilt down` where they show up mixed in the CLI
// output.
// https://github.com/kubernetes/kubernetes/blob/a4e5239bdc3d85f1f90c7811b03dc153d5121ffe/staging/src/k8s.io/client-go/discovery/cached/memory/memcache.go#L212-L221
// https://github.com/kubernetes/kubernetes/issues/92480
//
// informer errors when CRDs are removed
// https://github.com/kubernetes/kubernetes/issues/79610
//
// We're not convinced that controller-runtime logs EVER make sense to show to
// tilt users, so let's filter them out by default. Users can turn them on
// with --klog if they want.
func MaybeSetKlogOutput(w io.Writer) {
	if klogLevel == 0 {
		klog.SetOutput(io.Discard)
	} else {
		klog.SetOutput(w)
	}
}
