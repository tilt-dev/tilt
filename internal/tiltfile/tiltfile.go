package tiltfile

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"go.starlark.net/resolve"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

const FileName = "Tiltfile"
const unresourcedName = "k8s_yaml"

func init() {
	resolve.AllowLambda = true
	resolve.AllowNestedDef = true
	resolve.AllowGlobalReassign = true
}

// Load loads the Tiltfile in `filename`, and returns the manifests matching `matching`.
func Load(ctx context.Context, filename string, matching map[string]bool, logs io.Writer) (manifests []model.Manifest, global model.Manifest, configFiles []string, err error) {
	l := log.New(logs, "", log.LstdFlags)
	absFilename, err := ospath.RealAbs(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, model.Manifest{}, []string{filename}, fmt.Errorf("No Tiltfile found at path '%s'. Check out https://docs.tilt.build/tutorial.html", filename)
		}
		absFilename, _ = filepath.Abs(filename)
		return nil, model.Manifest{}, []string{absFilename}, err
	}

	tfRoot, _ := filepath.Split(absFilename)

	s := newTiltfileState(ctx, absFilename, tfRoot, l)
	defer func() {
		configFiles = s.configFiles
	}()

	if err := s.exec(); err != nil {
		if err, ok := err.(*starlark.EvalError); ok {
			return nil, model.Manifest{}, nil, errors.New(err.Backtrace())
		}
		return nil, model.Manifest{}, nil, err
	}

	resources, unresourced, err := s.assemble()
	if err != nil {
		return nil, model.Manifest{}, nil, err
	}

	if len(resources.k8s) > 0 {
		manifests, err = s.translateK8s(resources.k8s)
		if err != nil {
			return nil, model.Manifest{}, nil, err
		}
	} else {
		manifests, err = s.translateDC(resources.dc)
		if err != nil {
			return nil, model.Manifest{}, nil, err
		}
	}

	manifests, err = match(manifests, matching)
	if err != nil {
		return nil, model.Manifest{}, nil, err
	}

	var remainingYamlManifest model.Manifest
	if len(unresourced) > 0 {
		var podCreators []model.Manifest
		podCreators, remainingYamlManifest, err = k8sManifestsForPodCreators(ctx, unresourced)
		if err != nil {
			return nil, model.Manifest{}, nil, err
		}
		manifests = append(manifests, podCreators...)
	}

	// TODO(maia): `remainingYamlManifest` should be processed just like any
	// other manifest (i.e. get rid of "global yaml" concept)
	return manifests, remainingYamlManifest, s.configFiles, err
}

// k8sManifestsForPodCreators groups the given k8s entities into pod creator clusters
// (e.g. DeploymentA is a pod creator, and ServiceA would belong to its pod creator
// cluster b/c it applies to pods created by DeploymentA); returns a k8s-only manifest
// per pod creator cluster, and a single manifest containing the rest of the entities
// (will be deployed/displayed in HUD as a unit).
func k8sManifestsForPodCreators(ctx context.Context, entities []k8s.K8sEntity) (podCreators []model.Manifest,
	remaining model.Manifest, err error) {
	withPodSpec, allRest, err := k8s.FilterByHasPodTemplateSpec(entities)
	if err != nil {
		return nil, model.Manifest{}, err
	}

	for _, e := range withPodSpec {
		podTemplates, err := k8s.ExtractPodTemplateSpec(e)
		if err != nil {
			return nil, model.Manifest{}, errors.Wrap(err, "extracting pod template spec")
		}
		if len(podTemplates) == 0 {
			return nil, model.Manifest{}, errors.Wrap(err, "expected at least one pod template spec, got none")
		}

		if len(podTemplates) > 1 {
			logger.Get(ctx).Infof("Found multiple pod templates on %s, "+
				"looking for other entities that match the first one", e.ResourceName())
		}
		template := podTemplates[0]

		match, rest, err := k8s.FilterByLabels(allRest, template.Labels)
		if err != nil {
			return nil, model.Manifest{}, errors.Wrap(err, "filtering entities by label")
		}

		entityGroup := append([]k8s.K8sEntity{e}, match...)
		m, err := k8s.NewK8sOnlyManifest(model.ManifestName(e.Name()), entityGroup)
		if err != nil {
			return nil, model.Manifest{}, err
		}
		podCreators = append(podCreators, m)

		allRest = rest
	}

	remaining, err = k8s.NewK8sOnlyManifest(unresourcedName, allRest)

	return podCreators, remaining, nil
}

func skylarkStringDictToGoMap(d *starlark.Dict) (map[string]string, error) {
	r := map[string]string{}

	for _, tuple := range d.Items() {
		kV, ok := tuple[0].(starlark.String)
		if !ok {
			return nil, fmt.Errorf("key is not a string: %T (%v)", tuple[0], tuple[0])
		}

		k := string(kV)

		vV, ok := tuple[1].(starlark.String)
		if !ok {
			return nil, fmt.Errorf("value is not a string: %T (%v)", tuple[1], tuple[1])
		}

		v := string(vV)

		r[k] = v
	}

	return r, nil
}
