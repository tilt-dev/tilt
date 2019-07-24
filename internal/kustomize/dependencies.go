/**
Code for parsing Kustomize YAML and analyzing dependencies.

Adapted from
https://github.com/GoogleContainerTools/skaffold/blob/511c77f1736b657415500eb9b820ae7e4f753347/pkg/skaffold/deploy/kustomize.go

Copyright 2018 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kustomize

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"
	"sigs.k8s.io/kustomize/pkg/pgmconfig"
	"sigs.k8s.io/kustomize/pkg/types"
)

// Mostly taken from the [kustomize source code](https://github.com/kubernetes-sigs/kustomize/blob/ee68a9c450bc884b0d657fb7e3d62eb1ac59d14f/pkg/target/kusttarget.go#L97) itself.
func loadKustFile(dir string) ([]byte, string, error) {
	var content []byte
	var path string
	match := 0
	for _, kf := range pgmconfig.KustomizationFileNames {
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
			pgmconfig.KustomizationFileNames, dir)
	case 1:
		return content, path, nil
	default:
		return nil, "", fmt.Errorf(
			"Found multiple kustomization files under: %s\n", dir)
	}
}

// Code for parsing Kustomize adapted from Kustomize
// https://github.com/kubernetes-sigs/kustomize/blob/ee68a9c450bc884b0d657fb7e3d62eb1ac59d14f/pkg/target/kusttarget.go#L97
//
// Code for parsing out dependencies copied from Skaffold
// https://github.com/GoogleContainerTools/skaffold/blob/511c77f1736b657415500eb9b820ae7e4f753347/pkg/skaffold/deploy/kustomize.go
func dependenciesForKustomization(dir string) ([]string, error) {
	var deps []string

	buf, path, err := loadKustFile(dir)
	if err != nil {
		return nil, err
	}

	buf = types.FixKustomizationPreUnmarshalling(buf)

	content := types.Kustomization{}
	if err := yaml.Unmarshal(buf, &content); err != nil {
		return nil, err
	}

	errs := content.EnforceFields()
	if len(errs) > 0 {
		return nil, fmt.Errorf("Failed to read kustomization file under %s:\n"+strings.Join(errs, "\n"), dir)
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
	for _, patch := range content.PatchesStrategicMerge {
		deps = append(deps, filepath.Join(dir, string(patch)))
	}
	deps = append(deps, joinPaths(dir, content.Crds)...)
	for _, patch := range content.PatchesJson6902 {
		deps = append(deps, filepath.Join(dir, patch.Path))
	}
	for _, generator := range content.ConfigMapGenerator {
		deps = append(deps, joinPaths(dir, generator.FileSources)...)
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
