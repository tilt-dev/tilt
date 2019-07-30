package testdata

import (
	"path/filepath"
	"runtime"
)

func NginxIngressChartPath() string {
	return filepath.Join(staticPath(), "nginx-ingress-0.31.0.tgz")
}

func staticPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("Could not locate path to tiltfile/testdata")
	}

	return filepath.Dir(file)
}
