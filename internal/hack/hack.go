//go:build hack
// +build hack

// A hack to make sure code-generator is vendored
package hack

import (
	_ "k8s.io/code-generator"
)
