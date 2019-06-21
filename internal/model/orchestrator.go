package model

// The current orchestrator we're running with (K8s or DockerCompose)
type Orchestrator string

const OrchestratorUnknown = Orchestrator("")
const OrchestratorK8s = Orchestrator("Kubernetes")
const OrchestratorDC = Orchestrator("DockerCompose")
