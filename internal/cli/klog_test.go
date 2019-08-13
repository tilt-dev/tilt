package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

func TestResourceVersionTooOldWarningsSilenced(t *testing.T) {
	initKlog()

	out := bytes.NewBuffer(nil)
	klog.SetOutput(out)

	cache.PrintWatchEndedV4()
	klog.Flush()
	assert.Equal(t, "", out.String())

	cache.PrintWatchEndedWarning()
	klog.Flush()
	assert.Contains(t, out.String(), "reflector.go")
	assert.Contains(t, out.String(), "watch ended")
}

func TestResourceVersionTooOldWarningsPrinted(t *testing.T) {
	klogLevel = 5
	defer func() {
		klogLevel = 0
	}()
	initKlog()

	out := bytes.NewBuffer(nil)
	klog.SetOutput(out)

	cache.PrintWatchEndedV4()
	klog.Flush()
	assert.Contains(t, out.String(), "watch ended")
}
