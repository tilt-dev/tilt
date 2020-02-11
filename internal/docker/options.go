package docker

import "io"

type BuildOptions struct {
	Context     io.Reader
	Dockerfile  string
	Remove      bool
	BuildArgs   map[string]*string
	Target      string
	SSHSpecs    []string
	SecretSpecs []string
	Network     string
	ExtraTags   []string
}
