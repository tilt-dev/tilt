package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/klog/v2"
)

func TestResourceVersionTooOldWarningsSilenced(t *testing.T) {
	out := bytes.NewBuffer(nil)
	initKlog(out)

	PrintWatchEndedV4()
	klog.Flush()
	assert.Equal(t, "", out.String())

	PrintWatchEndedWarning()
	klog.Flush()
	assert.Contains(t, out.String(), "klog_test.go")
	assert.Contains(t, out.String(), "watch ended")
}

func TestResourceVersionTooOldWarningsPrinted(t *testing.T) {
	klogLevel = 5
	defer func() {
		klogLevel = 0
	}()
	out := bytes.NewBuffer(nil)
	initKlog(out)

	PrintWatchEndedV4()
	klog.Flush()
	assert.Contains(t, out.String(), "watch ended")
}

func TestEmptyGroupVersionErrorsSilenced(t *testing.T) {
	out := bytes.NewBuffer(nil)
	initKlog(out)

	klog.Error("couldn't get resource list for external.metrics.k8s.io/v1beta1: Got empty response for: external.metrics.k8s.io/v1beta1")
	klog.Flush()

	assert.Empty(t, out.String())
}

func PrintWatchEndedV4() {
	klog.V(4).Infof("watch ended")
}
func PrintWatchEndedWarning() {
	klog.Warningf("watch ended")
}
