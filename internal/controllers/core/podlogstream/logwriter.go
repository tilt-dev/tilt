package podlogstream

import (
	"bytes"
	"fmt"
	"io"
)

var failureMsg = []byte("failed to create fsnotify watcher: too many open files")
var newline = []byte("\n")

// Kubernetes has a bug where it will dump certain kinds of errors to the
// pod log stream.
// https://github.com/tilt-dev/tilt/issues/2487
type errorCapturingWriter struct {
	underlying io.Writer

	newlineTerminated bool
	errorTerminated   string
}

func (w *errorCapturingWriter) Write(p []byte) (n int, err error) {
	w.newlineTerminated = bytes.HasSuffix(p, newline)
	if bytes.HasSuffix(p, failureMsg) {
		w.errorTerminated =
			fmt.Sprintf("%s. Consider adjusting inotify limits: https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files",
				string(failureMsg))
	} else {
		w.errorTerminated = ""
	}

	return w.underlying.Write(p)
}
