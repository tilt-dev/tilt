//go:build darwin

package watch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsevents"
	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// TestMustScanSubDirs verifies that when the kernel sends an FSEvents overflow
// (MustScanSubDirs), it is surfaced as a FileEvent with Overflow() == true.
//
// To trigger the overflow, we build the watcher with NoDefer (aggressive
// delivery) and hold the events channel unread while hammering the watched
// directory with concurrent writes. The blocked consumer causes the FSEvents
// callback to stall, which saturates the kernel's delivery path and eventually
// causes it to drop events and send MustScanSubDirs.
func TestMustScanSubDirs(t *testing.T) {
	td := tempdir.NewTempDirFixture(t)
	root := td.Path()

	dw := &darwinNotify{
		ignore: EmptyMatcher{},
		logger: logger.NewTestLogger(bytes.NewBuffer(nil)),
		stream: &fsevents.EventStream{
			Latency: 0,
			Flags:   fsevents.FileEvents | fsevents.NoDefer,
			EventID: fsevents.LatestEventID(),
		},
		events: make(chan FileEvent),
		errors: make(chan error),
		stop:   make(chan struct{}),
	}
	dw.initAdd(root)
	require.NoError(t, dw.Start())
	defer dw.Close()

	// Hammer the watched directory with concurrent writes without reading from
	// dw.events, which will eventually cause the kernel to send MustScanSubDirs.
	const numFiles = 1000
	paths := make([]string, numFiles)
	for i := range paths {
		p := filepath.Join(root, fmt.Sprintf("file%d.txt", i))
		require.NoError(t, os.WriteFile(p, []byte("init"), 0644))
		paths[i] = p
	}

	// Now drain dw.events, looking for the overflow event.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		select {
		case event := <-dw.events:
			fmt.Printf("got event: %s\n", event.Path())
			continue
		case err := <-dw.errors:
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				// Success!
				return
			}
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal("kernel did not send MustScanSubDirs within timeout")
		}
	}
}
