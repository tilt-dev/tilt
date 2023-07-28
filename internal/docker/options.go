package docker

import (
	"io"

	"github.com/moby/buildkit/session/filesync"
)

type BuildOptions struct {
	Context            io.Reader
	Dockerfile         string
	Remove             bool
	BuildArgs          map[string]*string
	Target             string
	SSHSpecs           []string
	SecretSpecs        []string
	Network            string
	CacheFrom          []string
	PullParent         bool
	Platform           string
	ExtraTags          []string
	ForceLegacyBuilder bool
	DirSource          filesync.DirSource
	ExtraHosts         []string
}
