package config

import (
	"fmt"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
	"github.com/windmilleng/tilt/pkg/model"
)

func setEnabledResources(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var slResources starlark.Sequence
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"resources",
		&slResources,
	)
	if err != nil {
		return starlark.None, err
	}

	resources, err := value.SequenceToStringSlice(slResources)
	if err != nil {
		return starlark.None, errors.Wrap(err, "resources must be a list of string")
	}

	var mns []model.ManifestName
	for _, r := range resources {
		mns = append(mns, model.ManifestName(r))
	}

	err = starkit.SetState(thread, func(settings Settings) Settings {
		settings.enabledResources = mns
		return settings
	})
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// for the given args and list of full manifests, figure out which manifests the user actually selected
func (s Settings) EnabledResources(manifests []model.Manifest) ([]model.Manifest, error) {
	// if the user called set_enabled_resources, that trumps everything
	if s.enabledResources != nil {
		return match(manifests, s.enabledResources)
	}

	args := s.userConfigState.Args

	// if the user has not called config.parse and has specified args, use those to select which resources
	if args != nil && !s.configParseCalled {
		var mns []model.ManifestName
		for _, arg := range args {
			mns = append(mns, model.ManifestName(arg))
		}
		return match(manifests, mns)
	}

	// otherwise, they get all resources
	return manifests, nil
}

// add `manifestToAdd` and all of its transitive deps to `result`
func addManifestAndDeps(result map[model.ManifestName]bool, allManifestsByName map[model.ManifestName]model.Manifest, manifestToAdd model.ManifestName) {
	if result[manifestToAdd] {
		return
	}
	result[manifestToAdd] = true
	for _, dep := range allManifestsByName[manifestToAdd].ResourceDependencies {
		addManifestAndDeps(result, allManifestsByName, dep)
	}
}

// If the user requested only a subset of manifests, get just those manifests
func match(manifests []model.Manifest, requestedManifests []model.ManifestName) ([]model.Manifest, error) {
	if len(requestedManifests) == 0 {
		return manifests, nil
	}

	manifestsByName := make(map[model.ManifestName]model.Manifest)
	for _, m := range manifests {
		manifestsByName[m.Name] = m
	}

	manifestsToRun := make(map[model.ManifestName]bool)
	var unknownNames []string

	for _, m := range requestedManifests {
		if _, ok := manifestsByName[m]; !ok {
			unknownNames = append(unknownNames, string(m))
			continue
		}

		addManifestAndDeps(manifestsToRun, manifestsByName, m)
	}

	var result []model.Manifest
	for _, m := range manifests {
		if manifestsToRun[m.Name] {
			result = append(result, m)
		}
	}

	if len(unknownNames) > 0 {
		unmatchedNames := unmatchedManifestNames(manifests, requestedManifests)

		return nil, fmt.Errorf(`You specified some resources that could not be found: %s
Is this a typo? Existing resources in Tiltfile: %s`,
			sliceutils.QuotedStringList(unknownNames),
			sliceutils.QuotedStringList(unmatchedNames))
	}

	return result, nil
}

func unmatchedManifestNames(manifests []model.Manifest, requestedManifests []model.ManifestName) []string {
	requestedManifestsByName := make(map[model.ManifestName]bool)
	for _, m := range requestedManifests {
		requestedManifestsByName[m] = true
	}

	var ret []string
	for _, m := range manifests {
		if _, ok := requestedManifestsByName[m.Name]; !ok {
			ret = append(ret, string(m.Name))
		}
	}

	return ret
}
