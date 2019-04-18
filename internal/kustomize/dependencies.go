package kustomize

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

var kustomizationFileNames = []string{
	"kustomization.yaml",
	"kustomization.yml",
	"Kustomization",
}

// kustomization is the content of a kustomization.yaml file.
type kustomization struct {
	Bases              []string             `yaml:"bases"`
	Resources          []string             `yaml:"resources"`
	Patches            []string             `yaml:"patches"`
	CRDs               []string             `yaml:"crds"`
	PatchesJSON6902    []patchJSON6902      `yaml:"patchesJson6902"`
	ConfigMapGenerator []configMapGenerator `yaml:"configMapGenerator"`
}

type patchJSON6902 struct {
	Path string `yaml:"path"`
}

type configMapGenerator struct {
	Files []string `yaml:"files"`
}

// Mostly taken from the [kustomize source code](https://github.com/kubernetes-sigs/kustomize/blob/ee68a9c450bc884b0d657fb7e3d62eb1ac59d14f/pkg/target/kusttarget.go#L97) itself.
func loadKustFile(dir string) ([]byte, string, error) {
	var content []byte
	var path string
	match := 0
	for _, kf := range kustomizationFileNames {
		p := filepath.Join(dir, kf)
		c, err := ioutil.ReadFile(p)
		if err == nil {
			path = p
			match += 1
			content = c
		}
	}

	switch match {
	case 0:
		return nil, "", fmt.Errorf(
			"unable to find one of %v in directory '%s'",
			kustomizationFileNames, dir)
	case 1:
		return content, path, nil
	default:
		return nil, "", fmt.Errorf(
			"Found multiple kustomization files under: %s\n", dir)
	}
}

func dependenciesForKustomization(dir string) ([]string, error) {
	var deps []string

	buf, path, err := loadKustFile(dir)
	if err != nil {
		return nil, err
	}

	content := kustomization{}
	if err := yaml.Unmarshal(buf, &content); err != nil {
		return nil, err
	}

	for _, base := range content.Bases {
		baseDeps, err := dependenciesForKustomization(filepath.Join(dir, base))
		if err != nil {
			return nil, err
		}

		deps = append(deps, baseDeps...)
	}

	deps = append(deps, path)
	deps = append(deps, joinPaths(dir, content.Resources)...)
	deps = append(deps, joinPaths(dir, content.Patches)...)
	deps = append(deps, joinPaths(dir, content.CRDs)...)
	for _, patch := range content.PatchesJSON6902 {
		deps = append(deps, filepath.Join(dir, patch.Path))
	}
	for _, generator := range content.ConfigMapGenerator {
		deps = append(deps, joinPaths(dir, generator.Files)...)
	}

	return deps, nil
}

func Deps(baseDir string) ([]string, error) {
	deps, err := dependenciesForKustomization(baseDir)
	if err != nil {
		return nil, err
	}

	return uniqDependencies(deps), nil
}

func joinPaths(root string, paths []string) []string {
	var list []string

	for _, path := range paths {
		list = append(list, filepath.Join(root, path))
	}

	return list
}

func uniqDependencies(deps []string) []string {
	seen := make(map[string]struct{}, len(deps))
	j := 0
	for _, v := range deps {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		deps[j] = v
		j++
	}

	return deps[:j]
}
