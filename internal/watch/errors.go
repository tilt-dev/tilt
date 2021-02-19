package watch

import (
	"fmt"
	"runtime"
	"strings"
)

const DetectedOverflowErrMsg = `It looks like the inotify event queue has overflowed. Check these instructions for how to raise the queue limit: https://facebook.github.io/watchman/docs/install#system-specific-preparation`

func IsWindowsShortReadError(err error) bool {
	return runtime.GOOS == "windows" && err != nil && strings.Contains(err.Error(), "short read")
}

func WindowsShortReadErrorMessage(err error) string {
	return fmt.Sprintf("Windows I/O overflow.\n"+
		"You may be able to fix this by setting the env var %s.\n"+
		"Current buffer size: %d\n"+
		"More details: https://github.com/tilt-dev/tilt/issues/3556\n"+
		"Caused by: %v",
		WindowsBufferSizeEnvVar,
		DesiredWindowsBufferSize(),
		err)
}

func DetectedOverflowErrorMessage(err error) string {
	return fmt.Sprintf("%s\nerror: %v", DetectedOverflowErrMsg, err)
}
