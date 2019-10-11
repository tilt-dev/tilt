package docker

import "io"

type BuildOptions struct {
	Context    io.Reader
	Dockerfile string
	Remove     bool
	BuildArgs  map[string]*string
	Tags       []string
	Target     string
}
