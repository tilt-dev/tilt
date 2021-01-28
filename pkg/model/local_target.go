package model

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/sliceutils"
)

type LocalTarget struct {
	Name      TargetName
	UpdateCmd Cmd      // e.g. `make proto`
	ServeCmd  Cmd      // e.g. `python main.py`
	Links     []Link   // zero+ links assoc'd with this resource (to be displayed in UIs)
	Deps      []string // a list of ABSOLUTE file paths that are dependencies of this target
	ignores   []Dockerignore

	repos []LocalGitRepo

	// Indicates that we should allow this to run in parallel with other
	// resources  (by default, this is presumed unsafe and is not allowed).
	AllowParallel bool

	// For testing MVP
	Tags   []string // eventually we might want tags to be more widespread -- stored on manifest maybe?
	IsTest bool     // does this target represent a Test?

	ReadinessProbe *v1.Probe
}

var _ TargetSpec = LocalTarget{}

func NewLocalTarget(name TargetName, updateCmd Cmd, serveCmd Cmd, deps []string) LocalTarget {
	return LocalTarget{
		Name:      name,
		UpdateCmd: updateCmd,
		Deps:      deps,
		ServeCmd:  serveCmd,
	}
}

func (lt LocalTarget) Empty() bool {
	return lt.UpdateCmd.Empty() && lt.ServeCmd.Empty()
}

func (lt LocalTarget) WithAllowParallel(val bool) LocalTarget {
	lt.AllowParallel = val
	return lt
}

func (lt LocalTarget) WithRepos(repos []LocalGitRepo) LocalTarget {
	lt.repos = append(append([]LocalGitRepo{}, lt.repos...), repos...)
	return lt
}

func (lt LocalTarget) WithIgnores(ignores []Dockerignore) LocalTarget {
	lt.ignores = ignores
	return lt
}

func (lt LocalTarget) WithLinks(links []Link) LocalTarget {
	lt.Links = links
	return lt
}

func (lt LocalTarget) WithIsTest(isTest bool) LocalTarget {
	lt.IsTest = isTest
	return lt
}

func (lt LocalTarget) WithTags(tags []string) LocalTarget {
	lt.Tags = tags
	return lt
}

func (lt LocalTarget) WithReadinessProbe(probeSpec *v1.Probe) LocalTarget {
	lt.ReadinessProbe = probeSpec
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
	if !lt.UpdateCmd.Empty() && lt.UpdateCmd.Dir == "" {
		return fmt.Errorf("[Validate] LocalTarget cmd missing workdir")
	}
	if !lt.ServeCmd.Empty() && lt.ServeCmd.Dir == "" {
		return fmt.Errorf("[Validate] LocalTarget serve_cmd missing workdir")
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
	return lt.ignores
}

func (lt LocalTarget) IgnoredLocalDirectories() []string {
	return nil
}
