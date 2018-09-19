package state

type Resources struct {
	Resources []K8sResource
}

type K8sResource struct {
	Name string
	Yaml string

	Watched  []WatchedPaths
	Commands []Cmd

	K8sStatus string // a description of the K8s status (TODO(dbentley): use real structs from k8s client)
	Addr      string // address to connect to the running instance

	Last    SpanID   // the last completed trace
	Running TraceID  // any currently running trace
	Queued  []string // files that have been modified but not yet running
}

type WatchedPath struct {
	RootPath string   // the root that's being watched
	SubPaths []string // only relevant for git repos
	Type     string   // e.g. "git" or "tiltfile" or "yaml"

	// TODO(dbentley): ignores
}

type Cmd struct {
	Args string
	// TODO(dbentley): how to describe triggers?
}

type SpanID int64

type Span struct {
	// set at creation time
	ID      SpanID
	Parent  SpanID
	Name    string
	Started time.Time

	// keys can be added but not updated
	Fields map[string]string

	// set when finished
	Ended time.Time
}
