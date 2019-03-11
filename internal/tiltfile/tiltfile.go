package tiltfile

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"go.starlark.net/resolve"
	"go.starlark.net/starlark"

	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/k8s"
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

type TiltfileLoader interface {
	Load(ctx context.Context, filename string, matching map[string]bool, logs io.Writer) (manifests []model.Manifest, global model.Manifest, configFiles []string, warnings []string, err error)
}

type FakeTiltfileLoader struct {
	Manifests   []model.Manifest
	Global      model.Manifest
	ConfigFiles []string
	Warnings    []string
	Err         error
}

func NewFakeTiltfileLoader() *FakeTiltfileLoader {
	return &FakeTiltfileLoader{}
}

func (tfl *FakeTiltfileLoader) Load(ctx context.Context, filename string, matching map[string]bool, logs io.Writer) (manifests []model.Manifest, global model.Manifest, configFiles []string, warnings []string, err error) {
	return tfl.Manifests, tfl.Global, tfl.ConfigFiles, tfl.Warnings, tfl.Err
}

func NewTiltfileLoader() TiltfileLoader {
	return tiltfileLoader{analytics: analytics.NewMemoryAnalytics()}
}

func NewTiltfileLoaderWithAnalytics(analytics analytics.Analytics) TiltfileLoader {
	return tiltfileLoader{analytics: analytics}
}

type tiltfileLoader struct {
	analytics analytics.Analytics
}

// Load loads the Tiltfile in `filename`, and returns the manifests matching `matching`.
func (tfl tiltfileLoader) Load(ctx context.Context, filename string, matching map[string]bool, logs io.Writer) (manifests []model.Manifest, global model.Manifest, configFiles []string, warnings []string, err error) {
	l := log.New(logs, "", log.LstdFlags)
	absFilename, err := ospath.RealAbs(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, model.Manifest{}, []string{filename}, nil, fmt.Errorf("No Tiltfile found at path '%s'. Check out https://docs.tilt.dev/tutorial.html", filename)
		}
		absFilename, _ = filepath.Abs(filename)
		return nil, model.Manifest{}, []string{absFilename}, nil, err
	}

	tfRoot, _ := filepath.Split(absFilename)

	s := newTiltfileState(ctx, absFilename, tfRoot, l)
	defer func() {
		configFiles = s.configFiles
	}()

	s.logger.Printf("Beginning Tiltfile execution")
	if err := s.exec(); err != nil {
		if err, ok := err.(*starlark.EvalError); ok {
			return nil, model.Manifest{}, nil, s.warnings, errors.New(err.Backtrace())
		}
		return nil, model.Manifest{}, nil, s.warnings, err
	}

	resources, unresourced, err := s.assemble()
	if err != nil {
		return nil, model.Manifest{}, nil, s.warnings, err
	}

	if len(resources.k8s) > 0 {
		manifests, err = s.translateK8s(resources.k8s)
		if err != nil {
			return nil, model.Manifest{}, nil, s.warnings, err
		}
	} else {
		manifests, err = s.translateDC(resources.dc)
		if err != nil {
			return nil, model.Manifest{}, nil, s.warnings, err
		}
	}

	manifests, err = match(manifests, matching)
	if err != nil {
		return nil, model.Manifest{}, nil, s.warnings, err
	}

	yamlManifest := model.Manifest{}
	if len(unresourced) > 0 {
		yamlManifest, err = k8s.NewK8sOnlyManifest(unresourcedName, unresourced)
		if err != nil {
			return nil, model.Manifest{}, nil, s.warnings, err
		}
	}

	if err == nil {
		s.logger.Printf("Successfully loaded Tiltfile")
	}

	tfl.reportTiltfileLoaded(s.builtinCallCounts)

	// TODO(maia): `yamlManifest` should be processed just like any
	// other manifest (i.e. get rid of "global yaml" concept)
	return manifests, yamlManifest, s.configFiles, s.warnings, err
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

// If `v` is a `starlark.Sequence`, return a slice of its elements
// Otherwise, return it as a single-element slice
// For functions that take `Union[List[T], T]`
func starlarkValueOrSequenceToSlice(v starlark.Value) []starlark.Value {
	if v, ok := v.(starlark.Sequence); ok {
		var ret []starlark.Value
		it := v.Iterate()
		defer it.Done()
		var i starlark.Value
		for it.Next(&i) {
			ret = append(ret, i)
		}
		return ret
	}
	return []starlark.Value{v}
}

func (tfl *tiltfileLoader) reportTiltfileLoaded(counts map[string]int) {
	tags := make(map[string]string)
	for builtinName, count := range counts {
		tags[fmt.Sprintf("tiltfile.invoked.%s", builtinName)] = strconv.Itoa(count)
	}
	tfl.analytics.Incr("tiltfile.loaded", tags)
}
