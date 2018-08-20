package build

import (
	"fmt"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

type Dockerfile string

func DockerfileFromExisting(existing digest.Digest) Dockerfile {
	return Dockerfile(fmt.Sprintf("FROM %s", existing.Encoded()))
}

func (d Dockerfile) join(s string) Dockerfile {
	return Dockerfile(fmt.Sprintf("%s\n%s", d, s))
}

func (d Dockerfile) AddAll() Dockerfile {
	return d.join("ADD . /")
}

func (d Dockerfile) AddRun(dnePaths []pathMapping) Dockerfile {
	var newDf string
	if len(dnePaths) > 0 {
		// Add 'rm' statements; if changed file was deleted locally, remove if from container
		rmCmd := strings.Builder{}
		rmCmd.WriteString("rm") // sh -c?
		for _, p := range dnePaths {
			rmCmd.WriteString(fmt.Sprintf(" %s", p.ContainerPath))
		}
		newDf = fmt.Sprintf("%s\nRUN %s", newDf, rmCmd.String())
	}
	return d.join(newDf)
}

// NOTE(maia): can put more logic in here sometime; currently just returns an error
// if Dockerfile contains an ENTRYPOINT, which is illegal in Tilt right now (an
// ENTRYPOINT overrides a ContainerCreate Cmd, which we rely on).
// TODO: extract the ENTRYPOINT line from the Dockerfile and reapply it later.
func (d Dockerfile) Validate() error {
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
