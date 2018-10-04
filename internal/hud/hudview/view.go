package hudview

import "time"

type Resource struct {
	Name                    string
	DirectoryWatched        string
	LatestFileChanges       []string
	TimeSinceLastFileChange time.Duration
	Status                  ResourceStatus

	// e.g., "CrashLoopBackOff", "No Pod found", "Build error"
	StatusDesc string
}

type ResourceStatus int

const (
	// something is wrong and requires investigation, e.g. the build failed or the pod is crashlooping
	ResourceStatusBroken ResourceStatus = iota
	// tilt has observed changes since the last deploy, and is in the process of rebuilding and deploying
	ResourceStatusStale
	// the latest code is currently running
	ResourceStatusFresh
)

type View struct {
	Resources []Resource
}
