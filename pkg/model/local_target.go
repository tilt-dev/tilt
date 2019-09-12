package model

import (
	"github.com/windmilleng/tilt/internal/sliceutils"
)

// ✨ TODO ✨ -- should implement engine.WatchableTarget (but we can't assert that here)
type LocalTarget struct {
	Cmd  Cmd
	Deps []string
}

var _ TargetSpec = LocalTarget{}

func NewLocalTarget(cmd Cmd, deps []string) LocalTarget {
	return LocalTarget{cmd, deps}
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
	return nil
}

// Implements: engine.WatchableManifest
func (lt LocalTarget) Dependencies() []string {
	return sliceutils.DedupedAndSorted(lt.Deps)
}
func (lt LocalTarget) LocalRepos() []LocalGitRepo {
	return nil
}
func (lt LocalTarget) Dockerignores() []Dockerignore {
	return nil
}
func (lt LocalTarget) IgnoredLocalDirectories() []string {
	return nil
}
