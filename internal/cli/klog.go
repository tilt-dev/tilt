package cli

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"

	"k8s.io/klog"
)

// https://kubernetes.io/docs/reference/kubectl/cheatsheet/#kubectl-output-verbosity-and-debugging
var klogLevel = 0

type filteredWriter struct {
	underlying io.Writer
	filterFunc func(s string) bool
	leftover   []byte
}

func (fw *filteredWriter) Write(buf []byte) (int, error) {
	buf = append(fw.leftover, buf...)
	start := 0
	written := 0
	for i, b := range buf {
		if b == '\n' {
			end := i
			if buf[i-1] == '\r' {
				end--
			}
			s := string(buf[start:end])

			if !fw.filterFunc(s) {
				n, err := fw.underlying.Write(buf[start : i+1])
				written += n
				if err != nil {
					fw.leftover = append([]byte{}, buf[i+1:]...)
					return written, err
				}
			}

			start = i + 1
		}
	}

	fw.leftover = append([]byte{}, buf[start:]...)

	return written, nil
}

// lines matching `filterFunc` will not be output to the underlying writer
func newFilteredWriter(underlying io.Writer, filterFunc func(s string) bool) io.Writer {
	return &filteredWriter{underlying: underlying, filterFunc: filterFunc}
}

// everything on google indicates this warning is useless and should be ignored
// https://github.com/kubernetes/kubernetes/issues/22024
var isResourceVersionTooOldRegexp = regexp.MustCompile("reflector.go.*watch of.*ended with: too old resource version")

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
		w = newFilteredWriter(w, isResourceVersionTooOldRegexp.MatchString)
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
