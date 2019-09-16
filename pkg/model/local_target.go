package model

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/sliceutils"
)

type LocalTarget struct {
	Cmd  Cmd
	Deps []string

	dockerignores []Dockerignore
	repos         []LocalGitRepo
}

var _ TargetSpec = LocalTarget{}

func NewLocalTarget(cmd Cmd, deps []string) LocalTarget {
	return LocalTarget{Cmd: cmd, Deps: deps}
}

func (lt LocalTarget) WithRepos(repos []LocalGitRepo) LocalTarget {
	lt.repos = append(append([]LocalGitRepo{}, lt.repos...), repos...)
	return lt
}

func (lt LocalTarget) WithDockerignores(dockerignores []Dockerignore) LocalTarget {
	lt.dockerignores = append(append([]Dockerignore{}, lt.dockerignores...), dockerignores...)
	return lt
}

func (lt LocalTarget) ID() TargetID {
	return TargetID{
		Name: TargetName(lt.Cmd.String()),
		Type: TargetTypeLocal,
	}
}

func (lt LocalTarget) DependencyIDs() []TargetID {
	return nil
}

func (lt LocalTarget) Validate() error {
	if lt.Cmd.Empty() {
		return fmt.Errorf("[Validate] LocalTarget missing command")
	}
	return nil
}

// Implements: engine.WatchableManifest
func (lt LocalTarget) Dependencies() []string {
	return sliceutils.DedupedAndSorted(lt.Deps)
}

func (lt LocalTarget) LocalRepos() []LocalGitRepo {
	return lt.repos
}

func (lt LocalTarget) Dockerignores() []Dockerignore {
	return lt.dockerignores
}

func (lt LocalTarget) IgnoredLocalDirectories() []string {
	return nil
}
