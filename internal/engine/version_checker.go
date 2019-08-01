package engine

import (
	"context"
	"strings"
	"time"

	"github.com/windmilleng/tilt/internal/github"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
)

const githubOrg = "windmilleng"
const githubProject = "tilt"
const versionCheckInterval = time.Hour * 4

type TiltVersionChecker struct {
	started       bool
	clientFactory GithubClientFactory
	client        github.Client
	timerMaker    timerMaker
}

func NewTiltVersionChecker(ghcf GithubClientFactory, timerMaker timerMaker) *TiltVersionChecker {
	return &TiltVersionChecker{clientFactory: ghcf, timerMaker: timerMaker}
}

type GithubClientFactory func() github.Client

func NewGithubClientFactory() GithubClientFactory {
	return github.NewClient
}

func (tvc *TiltVersionChecker) OnChange(ctx context.Context, st store.RStore) {
	if tvc.started {
		return
	}

	s := st.RLockState()
	defer st.RUnlockState()
	if s.TiltBuildInfo.Dev {
		return
	}

	tvc.client = tvc.clientFactory()
	tvc.started = true

	go func() {
		for {
			version, err := tvc.client.GetLatestRelease(ctx, githubOrg, githubProject)
			if err != nil {
				if strings.Contains(err.Error(), "API rate limit exceeded") {
					logger.Get(ctx).Verbosef("error checking github for latest Tilt release: %v", err)
				} else {
					logger.Get(ctx).Infof("error checking github for latest Tilt release: %s", err.Error())
				}
			} else {
				st.Dispatch(LatestVersionAction{version})
			}
			select {
			case <-tvc.timerMaker(versionCheckInterval):
			case <-ctx.Done():
				return
			}
		}
	}()
}
