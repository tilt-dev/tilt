package cli

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/require"
)

func TestFilteredWriter(t *testing.T) {
	w := &bytes.Buffer{}
	fw := filteredWriter(w, isResourceVersionTooOldMessage)

	_, err := io.WriteString(fw, "hello\n")
	require.NoError(t, err)
	_, err = io.WriteString(fw, "W0718 14:19:52.083428   83999 reflector.go:302] github.com/windmilleng/tilt/internal/k8s/watch.go:105: watch of *v1.Event ended with: The resourceVersion for the provided watch is too old.\n")
	require.NoError(t, err)
	_, err = io.WriteString(fw, "goodbye\n")
	require.NoError(t, err)

	expected := "hello\ngoodbye\n"

	timeout := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(timeout) {
		if w.String() == expected {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.Equal(t, expected, w.String())
}

type errorWriter struct {
	err error
}

func (ew errorWriter) Write(p []byte) (n int, err error) {
	return 0, ew.err
}

func TestFilteredWriterError(t *testing.T) {
	expected := errors.New("foobar")
	ew := errorWriter{expected}
	fw := filteredWriter(ew, func(s string) bool {
		return false
	})
	for _, s := range []string{"hello\n", "goodbye\n"} {
		_, err := fw.Write([]byte(s))
		if err != nil {
			require.Equal(t, expected, err)
			break
		}
	}
}
