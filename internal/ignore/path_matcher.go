package ignore

import (
	"context"

	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/git"
	"github.com/windmilleng/tilt/internal/model"
)

func CreateFilter(m model.Manifest) model.PathMatcher {
	matchers := []model.PathMatcher{}
	if m.TiltFilename != "" {
		m, err := model.NewSimpleFileMatcher(m.TiltFilename)
		if err == nil {
			matchers = append(matchers, m)
		}
	}
	for _, r := range m.Repos {
		gim, err := git.NewRepoIgnoreTester(context.Background(), r.LocalPath, r.GitignoreContents)
		if err == nil {
			matchers = append(matchers, gim)
		}

		dim, err := dockerignore.DockerIgnoreTesterFromContents(r.LocalPath, r.DockerignoreContents)
		if err == nil {
			matchers = append(matchers, dim)
		}
	}

	return model.NewCompositeMatcher(matchers)
}
