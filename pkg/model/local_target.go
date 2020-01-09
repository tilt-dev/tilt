package model

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/sliceutils"
)

type LocalTarget struct {
	Name      TargetName
	UpdateCmd Cmd      // e.g. `make proto`
	ServeCmd  Cmd      // e.g. `python main.py`
	Workdir   string   // directory from which the commands should be run
	deps      []string // a list of ABSOLUTE file paths that are dependencies of this target
	ignores   []Dockerignore

	repos []LocalGitRepo
}

var _ TargetSpec = LocalTarget{}

func NewLocalTarget(name TargetName, updateCmd Cmd, serveCmd Cmd, deps []string, workdir string) LocalTarget {
	return LocalTarget{
		Name:      name,
		UpdateCmd: updateCmd,
		Workdir:   workdir,
		deps:      deps,
		ServeCmd:  serveCmd,
	}
}

func (lt LocalTarget) Empty() bool {
	return lt.UpdateCmd.Empty() && lt.ServeCmd.Empty()
}

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
	if !lt.UpdateCmd.Empty() {
		if lt.Workdir == "" {
			return fmt.Errorf("[Validate] LocalTarget missing workdir")
		}
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
