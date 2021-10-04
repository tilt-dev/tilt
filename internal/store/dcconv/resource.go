package dcconv

// A DockerResource exposes a high-level status that summarizes
// the containers we care about in a DockerCompose session.
//
// Long-term, this may become an explicit API server object.
type DockerResource struct {
	ContainerID string
}
