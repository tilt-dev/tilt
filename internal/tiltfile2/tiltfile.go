package tiltfile2

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/skylark/resolve"
	"github.com/pkg/errors"
	skylark "go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

const FileName = "Tiltfile"
const unresourcedName = "k8s_yaml"

func init() {
	resolve.AllowLambda = true
	resolve.AllowNestedDef = true
}

// Load loads the Tiltfile in `filename`, and returns the manifests matching `matching`.
func Load(ctx context.Context, filename string, matching map[string]bool) (manifests []model.Manifest, global model.YAMLManifest, configFiles []string, err error) {
	absFilename, err := ospath.RealAbs(filename)
	if err != nil {
		absFilename, _ = filepath.Abs(filename)
		return nil, model.YAMLManifest{}, []string{absFilename}, err
	}

	tfRoot, _ := filepath.Split(absFilename)

	s := newTiltfileState(ctx, absFilename, tfRoot)
	defer func() {
		configFiles = s.configFiles
	}()

	if err := s.exec(); err != nil {
		if err, ok := err.(*skylark.EvalError); ok {
			return nil, model.YAMLManifest{}, nil, errors.Wrap(err, err.Backtrace())
		}
		return nil, model.YAMLManifest{}, nil, err
	}

	resources, unresourced, err := s.assemble()
	if err != nil {
		return nil, model.YAMLManifest{}, nil, err
	}

	if len(resources.k8s) > 0 {
		manifests, err = s.translateK8s(resources.k8s)
		if err != nil {
			return nil, model.YAMLManifest{}, nil, err
		}
	} else {
		manifests, err = s.translateDC(resources.dc)
		if err != nil {
			return nil, model.YAMLManifest{}, nil, err
		}
	}

	manifests, err = match(manifests, matching)
	if err != nil {
		return nil, model.YAMLManifest{}, nil, err
	}

	yamlManifest := model.YAMLManifest{}
	if len(unresourced) > 0 {
		yaml, err := k8s.SerializeYAML(unresourced)
		if err != nil {
			return nil, model.YAMLManifest{}, nil, err
		}

		yamlManifest = model.NewYAMLManifest(unresourcedName, yaml, nil)
	}

	return manifests, yamlManifest, s.configFiles, err
}

func skylarkStringDictToGoMap(d *skylark.Dict) (map[string]string, error) {
	r := map[string]string{}

	for _, tuple := range d.Items() {
		kV, ok := tuple[0].(skylark.String)
		if !ok {
			return nil, fmt.Errorf("key is not a string: %T (%v)", tuple[0], tuple[0])
		}

		k := string(kV)

		vV, ok := tuple[1].(skylark.String)
		if !ok {
			return nil, fmt.Errorf("value is not a string: %T (%v)", tuple[1], tuple[1])
		}

		v := string(vV)

		r[k] = v
	}

	return r, nil
}
