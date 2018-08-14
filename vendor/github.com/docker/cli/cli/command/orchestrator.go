package command

import (
	"fmt"
	"io"
	"os"
)

// Orchestrator type acts as an enum describing supported orchestrators.
type Orchestrator string

const (
	// OrchestratorKubernetes orchestrator
	OrchestratorKubernetes = Orchestrator("kubernetes")
	// OrchestratorSwarm orchestrator
	OrchestratorSwarm = Orchestrator("swarm")
	// OrchestratorAll orchestrator
	OrchestratorAll   = Orchestrator("all")
	orchestratorUnset = Orchestrator("unset")

	defaultOrchestrator           = OrchestratorSwarm
	envVarDockerStackOrchestrator = "DOCKER_STACK_ORCHESTRATOR"
	envVarDockerOrchestrator      = "DOCKER_ORCHESTRATOR"
)

// HasKubernetes returns true if defined orchestrator has Kubernetes capabilities.
func (o Orchestrator) HasKubernetes() bool {
	return o == OrchestratorKubernetes || o == OrchestratorAll
}

// HasSwarm returns true if defined orchestrator has Swarm capabilities.
func (o Orchestrator) HasSwarm() bool {
	return o == OrchestratorSwarm || o == OrchestratorAll
}

// HasAll returns true if defined orchestrator has both Swarm and Kubernetes capabilities.
func (o Orchestrator) HasAll() bool {
	return o == OrchestratorAll
}

func normalize(value string) (Orchestrator, error) {
	switch value {
	case "kubernetes":
		return OrchestratorKubernetes, nil
	case "swarm":
		return OrchestratorSwarm, nil
	case "":
		return orchestratorUnset, nil
	case "all":
		return OrchestratorAll, nil
	default:
		return defaultOrchestrator, fmt.Errorf("specified orchestrator %q is invalid, please use either kubernetes, swarm or all", value)
	}
}

// GetStackOrchestrator checks DOCKER_STACK_ORCHESTRATOR environment variable and configuration file
// orchestrator value and returns user defined Orchestrator.
func GetStackOrchestrator(flagValue, value string, stderr io.Writer) (Orchestrator, error) {
	// Check flag
	if o, err := normalize(flagValue); o != orchestratorUnset {
		return o, err
	}
	// Check environment variable
	env := os.Getenv(envVarDockerStackOrchestrator)
	if env == "" && os.Getenv(envVarDockerOrchestrator) != "" {
		fmt.Fprintf(stderr, "WARNING: experimental environment variable %s is set. Please use %s instead\n", envVarDockerOrchestrator, envVarDockerStackOrchestrator)
	}
	if o, err := normalize(env); o != orchestratorUnset {
		return o, err
	}
	// Check specified orchestrator
	if o, err := normalize(value); o != orchestratorUnset {
		return o, err
	}
	// Nothing set, use default orchestrator
	return defaultOrchestrator, nil
}
