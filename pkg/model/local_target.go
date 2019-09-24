package model

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/sliceutils"
)

type LocalTarget struct {
	Name    TargetName
	Cmd     Cmd
	Workdir string   // directory from which this Cmd should be run
	deps    []string // a list of ABSOLUTE file paths that are dependencies of this target

	repos []LocalGitRepo
}

var _ TargetSpec = LocalTarget{}

func NewLocalTarget(name TargetName, cmd Cmd, workdir string, deps []string) LocalTarget {
	return LocalTarget{
		Name:    name,
		Cmd:     cmd,
		Workdir: workdir,
		deps:    deps,
	}
}

func (lt LocalTarget) Empty() bool { return lt.Cmd.Empty() }

func (lt LocalTarget) WithRepos(repos []LocalGitRepo) LocalTarget {
	lt.repos = append(append([]LocalGitRepo{}, lt.repos...), repos...)
	return lt
}

func (lt LocalTarget) ID() TargetID {
	return TargetID{
		Name: lt.Name,
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
	return sliceutils.DedupedAndSorted(lt.deps)
}

func (lt LocalTarget) LocalRepos() []LocalGitRepo {
	return lt.repos
}

func (lt LocalTarget) Dockerignores() []Dockerignore {
	return nil
}

func (lt LocalTarget) IgnoredLocalDirectories() []string {
	return nil
}
