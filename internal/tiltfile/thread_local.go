package tiltfile

import (
	"errors"
	"fmt"

	"github.com/google/skylark"
)

const buildContextKey = "buildContext"
const readFilesKey = "readFiles"
const reposKey = "repos"
const globalYAMLKey = "globalYaml"
const globalYAMLDepsKey = "globalYamlDeps"

func setGlobalYAML(t *skylark.Thread, yaml string) {
	t.SetLocal(globalYAMLKey, yaml)
}

func setGlobalYAMLDeps(t *skylark.Thread, deps []string) {
	t.SetLocal(globalYAMLDepsKey, deps)
}

func getGlobalYAML(t *skylark.Thread) (string, error) {
	obj := t.Local(globalYAMLKey)
	if obj == nil {
		return "", nil
	}

	yaml, ok := obj.(string)
	if !ok {
		return "", fmt.Errorf(
			"internal error: %s thread local was not of type string", globalYAMLKey)
	}
	return yaml, nil
}

func getGlobalYAMLDeps(t *skylark.Thread) ([]string, error) {
	obj := t.Local(globalYAMLDepsKey)
	if obj == nil {
		return nil, nil
	}

	deps, ok := obj.([]string)
	if !ok {
		return nil, fmt.Errorf(
			"internal error: %s thread local was not of type []string", globalYAMLDepsKey)
	}
	return deps, nil
}

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

	var r []string
	readFilesMap := make(map[string]bool)
	for _, f := range readFiles {
		if !readFilesMap[f] {
			readFilesMap[f] = true
			r = append(r, f)
		}
	}
	return readFiles, nil
}

func (t *Tiltfile) recordReadFile(thread *skylark.Thread, path string) error {
	path = t.absPath(path)
	readFiles, err := getReadFiles(thread)
	if err != nil {
		return err
	}
	thread.SetLocal(readFilesKey, append(readFiles, path))
	return nil
}
