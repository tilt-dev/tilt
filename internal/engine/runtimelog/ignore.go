package runtimelog

import "github.com/tilt-dev/tilt/internal/container"

const IstioInitContainerName = container.Name("istio-init")
const IstioSidecarContainerName = container.Name("istio-proxy")
