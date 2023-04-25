package container

const IstioInitContainerName = Name("istio-init")
const IstioSidecarContainerName = Name("istio-proxy")

const LinkerdInitContainerName = Name("linkerd-init")
const LinkerdSidecarContainerName = Name("linkerd-proxy")
