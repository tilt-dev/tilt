package testdata

import (
	"path/filepath"
	"runtime"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/options"
)

// Returns a self-signed cert-key pair for use in tests.
func CertKey() options.GeneratableKeyCert {
	_, curFile, _, _ := runtime.Caller(0)
	return options.GeneratableKeyCert{
		FixtureDirectory: filepath.Dir(curFile),
	}
}
