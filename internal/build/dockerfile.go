package build

import (
	"fmt"
	"strings"

	digest "github.com/opencontainers/go-digest"
	"github.com/windmilleng/tilt/internal/model"
)

type Dockerfile string

// DockerfileFromExisting creates a new Dockerfile that uses the supplied image
// as its base image with a FROM statement. This is necessary for iterative
// Docker builds.
func DockerfileFromExisting(existing digest.Digest) Dockerfile {
	return Dockerfile(fmt.Sprintf("FROM %s", existing.Encoded()))
}

func (d Dockerfile) join(s string) Dockerfile {
	return Dockerfile(fmt.Sprintf("%s\n%s", d, s))
}

func (d Dockerfile) AddAll() Dockerfile {
	return d.join("ADD . /")
}

func (d Dockerfile) Run(cmd model.Cmd) Dockerfile {
	return d.join(cmd.RunStr())
}

func (d Dockerfile) Entrypoint(cmd model.Cmd) Dockerfile {
	return d.join(cmd.EntrypointStr())
}

func (d Dockerfile) RmPaths(pathsToRm []pathMapping) Dockerfile {
	var newDf string
	if len(pathsToRm) > 0 {
		// Add 'rm' statements; if changed file was deleted locally, remove if from container
		rmCmd := strings.Builder{}
		rmCmd.WriteString("rm") // sh -c?
		for _, p := range pathsToRm {
			rmCmd.WriteString(fmt.Sprintf(" %s", p.ContainerPath))
		}
		newDf = fmt.Sprintf("%s\nRUN %s", newDf, rmCmd.String())
	}
	return d.join(newDf)
}

func (d Dockerfile) ForbidEntrypoint() error {
	for _, line := range strings.Split(string(d), "\n") {
		if strings.HasPrefix(line, "ENTRYPOINT") {
			return ErrEntrypointInDockerfile
		}
	}
	return nil
}

func (d Dockerfile) String() string {
	return string(d)
}
