package tiltfile2

import (
	"context"
	"fmt"

	"github.com/google/skylark"
	"github.com/google/skylark/resolve"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

func init() {
	resolve.AllowLambda = true
	resolve.AllowNestedDef = true
}

func Load(ctx context.Context, filename string) ([]model.Manifest, model.YAMLManifest, []string, error) {
	filename, err := ospath.RealAbs(filename)
	if err != nil {
		return nil, model.YAMLManifest{}, nil, err
	}

	s := newTiltfileState(ctx, filename)

	if err := s.exec(); err != nil {
		return nil, model.YAMLManifest{}, nil, err
	}
	assembled, err := s.assemble()
	if err != nil {
		return nil, model.YAMLManifest{}, nil, err
	}
	manifests, err := s.translate(assembled)
	return manifests, model.YAMLManifest{}, s.configFiles, err
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
