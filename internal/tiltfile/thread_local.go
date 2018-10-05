package tiltfile

import (
	"errors"
	"path/filepath"

	"github.com/google/skylark"
)

const buildContextKey = "buildContext"
const readFilesKey = "readFiles"
const reposKey = "repos"

func getAndClearBuildContext(t *skylark.Thread) (*dockerImage, error) {
	obj := t.Local(buildContextKey)
	if obj == nil {
		return nil, nil
	}

	buildContext, ok := obj.(*dockerImage)
	if !ok {
		return nil, errors.New("internal error: buildContext thread local was not of type *dockerImage")
	}
	t.SetLocal(buildContextKey, nil)
	return buildContext, nil
}

func getAndClearReadFiles(t *skylark.Thread) ([]string, error) {
	readFiles, err := getReadFiles(t)
	t.SetLocal(readFilesKey, nil)
	if err != nil {
		return nil, err
	}
	return readFiles, nil
}

func getReadFiles(t *skylark.Thread) ([]string, error) {
	obj := t.Local(readFilesKey)
	if obj == nil {
		return nil, nil
	}

	readFiles, ok := obj.([]string)
	if !ok {
		return nil, errors.New("internal error: readFiles thread local was not of type []string")
	}
	return readFiles, nil
}

func recordReadFile(t *skylark.Thread, path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	readFiles, err := getReadFiles(t)
	if err != nil {
		return err
	}
	t.SetLocal(readFilesKey, append(readFiles, path))
	return nil
}

func getRepos(t *skylark.Thread) ([]gitRepo, error) {
	obj := t.Local(reposKey)
	if obj == nil {
		return []gitRepo{}, nil
	}

	repos, ok := obj.([]gitRepo)
	if !ok {
		return nil, errors.New("internal error: repos thread local was not of type []gitRepo")
	}

	return repos, nil
}

func addRepo(t *skylark.Thread, repo gitRepo) error {
	repos, err := getRepos(t)
	if err != nil {
		return err
	}

	t.SetLocal(reposKey, append(repos, repo))
	return nil
}
