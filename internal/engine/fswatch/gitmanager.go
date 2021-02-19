package fswatch

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type repoNotifyCancel struct {
	repo   model.LocalGitRepo
	notify watch.Notify
	cancel func()
}

// Watches git for branch switches and fires events.
type GitManager struct {
	fsWatcherMaker watch.FsWatcherMaker
	repoWatches    map[model.LocalGitRepo]repoNotifyCancel
}

func NewGitManager(fsWatcherMaker watch.FsWatcherMaker) *GitManager {
	return &GitManager{
		fsWatcherMaker: fsWatcherMaker,
		repoWatches:    make(map[model.LocalGitRepo]repoNotifyCancel),
	}
}

func (m *GitManager) diff(ctx context.Context, st store.RStore) (setup, teardown []model.LocalGitRepo) {
	state := st.RLockState()
	defer st.RUnlockState()

	if !state.EngineMode.WatchesFiles() {
		return nil, nil
	}

	watchable := WatchableTargetsForManifests(state.Manifests())
	reposToProcess := make(map[model.LocalGitRepo]bool)
	for _, w := range watchable {
		for _, repo := range w.LocalRepos() {
			reposToProcess[repo] = true
		}
	}

	for repo := range reposToProcess {
		if _, ok := m.repoWatches[repo]; !ok {
			setup = append(setup, repo)
		}
	}

	for repo := range m.repoWatches {
		if _, ok := reposToProcess[repo]; !ok {
			teardown = append(teardown, repo)
		}
	}

	return setup, teardown
}

func (m *GitManager) OnChange(ctx context.Context, st store.RStore) {
	setup, teardown := m.diff(ctx, st)

	for _, repo := range teardown {
		p, ok := m.repoWatches[repo]
		delete(m.repoWatches, repo)

		if !ok || p.notify == nil {
			continue
		}
		_ = p.notify.Close()
		p.cancel()
	}

	for _, repo := range setup {
		m.repoWatches[repo] = repoNotifyCancel{repo: repo}

		// Create a file watcher that watches .git/HEAD,
		// which is where the current branch is stored.
		// https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain
		//
		// Whenever we see a change, we will re-check the current git branch.
		watcher, err := m.fsWatcherMaker(
			[]string{gitHeadPath(repo)},
			watch.EmptyMatcher{},
			logger.Get(ctx))
		if err != nil {
			logger.Get(ctx).Debugf("Error making watcher for git branches: %s", repo.LocalPath)
			continue
		}

		err = watcher.Start()
		if err != nil {
			logger.Get(ctx).Debugf("Error watching git branches: %s", repo.LocalPath)
			continue
		}

		ctx, cancel := context.WithCancel(ctx)
		go m.dispatchBranchChangesLoop(ctx, repo, watcher, st)
		m.repoWatches[repo] = repoNotifyCancel{repo, watcher, cancel}
	}
}

func (m *GitManager) dispatchBranchChangesLoop(ctx context.Context, repo model.LocalGitRepo,
	watcher watch.Notify, st store.RStore) {
	m.dispatchGitBranchStatus(st, repo)

	for {
		select {
		case <-ctx.Done():
			return

		case _, ok := <-watcher.Events():
			if !ok {
				return
			}

			m.dispatchGitBranchStatus(st, repo)
		}
	}

}

func (m *GitManager) dispatchGitBranchStatus(st store.RStore, repo model.LocalGitRepo) {
	b, err := ioutil.ReadFile(gitHeadPath(repo))
	if err != nil {
		// ignore errors reading the file
		return
	}

	st.Dispatch(GitBranchStatusAction{
		Time: time.Now(),
		Repo: repo,
		Head: string(b),
	})
}

func gitHeadPath(repo model.LocalGitRepo) string {
	return filepath.Join(repo.LocalPath, ".git", "HEAD")
}
