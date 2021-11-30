package model

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type LocalTarget struct {
	UpdateCmdSpec *v1alpha1.CmdSpec

	Name     TargetName
	ServeCmd Cmd      // e.g. `python main.py`
	Links    []Link   // zero+ links assoc'd with this resource (to be displayed in UIs)
	Deps     []string // a list of ABSOLUTE file paths that are dependencies of this target
	ignores  []Dockerignore

	repos []LocalGitRepo

	// Indicates that we should allow this to run in parallel with other
	// resources  (by default, this is presumed unsafe and is not allowed).
	AllowParallel bool

	// For testing MVP
	Tags   []string // eventually we might want tags to be more widespread -- stored on manifest maybe?
	IsTest bool     // does this target represent a Test?

	ReadinessProbe *v1alpha1.Probe

	// Move this to CmdServerSpec when we move CmdServer to API
	ServeCmdDisableSource *v1alpha1.DisableSource
}

var _ TargetSpec = LocalTarget{}

func NewLocalTarget(name TargetName, updateCmd Cmd, serveCmd Cmd, deps []string) LocalTarget {
	var updateCmdSpec *v1alpha1.CmdSpec
	if !updateCmd.Empty() {
		updateCmdSpec = &v1alpha1.CmdSpec{
			Args: updateCmd.Argv,
			Dir:  updateCmd.Dir,
			Env:  updateCmd.Env,
		}
	}

	return LocalTarget{
		Name:          name,
		UpdateCmdSpec: updateCmdSpec,
		Deps:          deps,
		ServeCmd:      serveCmd,
	}
}

func (lt LocalTarget) UpdateCmdName() string {
	if lt.UpdateCmdSpec == nil {
		return ""
	}
	return apis.SanitizeName(fmt.Sprintf("%s:update", lt.ID().Name))
}

func (lt LocalTarget) Empty() bool {
	return lt.UpdateCmdSpec == nil && lt.ServeCmd.Empty()
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

func (lt LocalTarget) WithReadinessProbe(probeSpec *v1alpha1.Probe) LocalTarget {
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
	if lt.UpdateCmdSpec != nil && lt.UpdateCmdSpec.Dir == "" {
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
