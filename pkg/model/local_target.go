package model

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/sliceutils"
)

type LocalTarget struct {
	Name      TargetName
	UpdateCmd Cmd
	ServeCmd  Cmd
	Workdir   string   // directory from which this UpdateCmd should be run
	deps      []string // a list of ABSOLUTE file paths that are dependencies of this target
	ignores   []Dockerignore

	repos []LocalGitRepo
}

var _ TargetSpec = LocalTarget{}

func NewLocalTarget(name TargetName, updateCmd Cmd, workdir string, deps []string, serveCmd Cmd) LocalTarget {
	return LocalTarget{
		Name:      name,
		UpdateCmd: updateCmd,
		Workdir:   workdir,
		deps:      deps,
	}
}

func (lt LocalTarget) Empty() bool { return lt.UpdateCmd.Empty() }

func (lt LocalTarget) WithRepos(repos []LocalGitRepo) LocalTarget {
	lt.repos = append(append([]LocalGitRepo{}, lt.repos...), repos...)
	return lt
}

func (lt LocalTarget) WithIgnores(ignores []Dockerignore) LocalTarget {
	lt.ignores = ignores
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
	if lt.UpdateCmd.Empty() {
		return fmt.Errorf("[Validate] LocalTarget missing command")
	}
	if lt.Workdir == "" {
		return fmt.Errorf("[Validate] LocalTarget missing workdir")
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
	return lt.ignores
}

func (lt LocalTarget) IgnoredLocalDirectories() []string {
	return nil
}
