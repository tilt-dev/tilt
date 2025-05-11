//go:build tools
// +build tools

package hack

import (
	_ "k8s.io/code-generator/cmd/validation-gen"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
)
