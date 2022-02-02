package config

import (
	"fmt"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
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
		settings.disableAll = false
		settings.enabledResources = mns
		return settings
	})
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

func clearEnabledResources(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	err := starkit.SetState(thread, func(settings Settings) Settings {
		settings.disableAll = true
		return settings
	})
	return starlark.None, err
}

// for the given args and list of full manifests, figure out which manifests the user actually selected
func (s Settings) EnabledResources(tf *v1alpha1.Tiltfile, manifests []model.Manifest) ([]model.ManifestName, error) {
	if s.disableAll {
		return nil, nil
	}

	// by default, nil = match all resources
	var requestedManifests []model.ManifestName

	// if the user called set_enabled_resources, that trumps everything
	if s.enabledResources != nil {
		requestedManifests = s.enabledResources
	} else {
		args := tf.Spec.Args

		// if the user has not called config.parse and has specified args, use those to select which resources
		if args != nil && !s.configParseCalled {
			for _, arg := range args {
				requestedManifests = append(requestedManifests, model.ManifestName(arg))
			}
		}
	}

	return match(manifests, requestedManifests)
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
func match(manifests []model.Manifest, requestedManifests []model.ManifestName) ([]model.ManifestName, error) {
	if len(requestedManifests) == 0 {
		var result []model.ManifestName
		for _, m := range manifests {
			result = append(result, m.Name)
		}
		return result, nil
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

	var result []model.ManifestName
	for _, m := range manifests {
		// Default to including UnresourcedYAML ("Uncategorized") to match historical behavior.
		if manifestsToRun[m.Name] || m.Name == model.UnresourcedYAMLManifestName {
			result = append(result, m.Name)
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
