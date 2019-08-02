package tiltfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"go.starlark.net/resolve"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

const FileName = "Tiltfile"
const TiltIgnoreFileName = ".tiltignore"

func init() {
	resolve.AllowLambda = true
	resolve.AllowNestedDef = true
	resolve.AllowGlobalReassign = true
}

type TiltfileLoadResult struct {
	Manifests          []model.Manifest
	ConfigFiles        []string
	Warnings           []string
	TiltIgnoreContents string
	FeatureFlags       map[string]bool
	TeamName           string
}

func (r TiltfileLoadResult) Orchestrator() model.Orchestrator {
	for _, manifest := range r.Manifests {
		if manifest.IsK8s() {
			return model.OrchestratorK8s
		} else if manifest.IsDC() {
			return model.OrchestratorDC
		}
	}
	return model.OrchestratorUnknown
}

type TiltfileLoader interface {
	Load(ctx context.Context, filename string, matching map[string]bool) (TiltfileLoadResult, error)
}

type FakeTiltfileLoader struct {
	Result TiltfileLoadResult
	Err    error
}

var _ TiltfileLoader = &FakeTiltfileLoader{}

func NewFakeTiltfileLoader() *FakeTiltfileLoader {
	return &FakeTiltfileLoader{}
}

func (tfl *FakeTiltfileLoader) Load(ctx context.Context, filename string, matching map[string]bool) (TiltfileLoadResult, error) {
	return tfl.Result, tfl.Err
}

func ProvideTiltfileLoader(
	analytics *analytics.TiltAnalytics,
	kCli k8s.Client,
	dcCli dockercompose.DockerComposeClient,
	kubeContext k8s.KubeContext,
	fDefaults feature.Defaults) TiltfileLoader {
	return tiltfileLoader{
		analytics:   analytics,
		kCli:        kCli,
		dcCli:       dcCli,
		kubeContext: kubeContext,
		fDefaults:   fDefaults,
	}
}

type tiltfileLoader struct {
	analytics   *analytics.TiltAnalytics
	kCli        k8s.Client
	dcCli       dockercompose.DockerComposeClient
	kubeContext k8s.KubeContext
	fDefaults   feature.Defaults
}

var _ TiltfileLoader = &tiltfileLoader{}

func printWarnings(s *tiltfileState) {
	for _, w := range s.warnings {
		s.logger.Infof("WARNING: %s\n", w)
	}
}

// Load loads the Tiltfile in `filename`, and returns the manifests matching `matching`.
func (tfl tiltfileLoader) Load(ctx context.Context, filename string, matching map[string]bool) (tlr TiltfileLoadResult, err error) {
	absFilename, err := ospath.RealAbs(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return TiltfileLoadResult{ConfigFiles: []string{filename}}, fmt.Errorf("No Tiltfile found at paths '%s'. Check out https://docs.tilt.dev/tutorial.html", filename)
		}
		absFilename, _ = filepath.Abs(filename)
		return TiltfileLoadResult{ConfigFiles: []string{absFilename}}, err
	}

	privateRegistry := tfl.kCli.PrivateRegistry(ctx)
	s := newTiltfileState(ctx, tfl.dcCli, tfl.kubeContext, privateRegistry, feature.FromDefaults(tfl.fDefaults))
	printedWarnings := false
	defer func() {
		tlr.ConfigFiles = s.configFiles
		tlr.Warnings = s.warnings
		if !printedWarnings {
			printWarnings(s)
		}
	}()

	s.logger.Infof("Beginning Tiltfile execution")
	_, err = s.exec(absFilename)
	if err != nil {
		if err, ok := err.(*starlark.EvalError); ok {
			return TiltfileLoadResult{}, errors.New(err.Backtrace())
		}
		return TiltfileLoadResult{}, err
	}

	resources, unresourced, err := s.assemble()
	if err != nil {
		return TiltfileLoadResult{}, err
	}

	var manifests []model.Manifest

	if len(resources.k8s) > 0 {
		manifests, err = s.translateK8s(resources.k8s)
		if err != nil {
			return TiltfileLoadResult{}, err
		}
	} else {
		manifests, err = s.translateDC(resources.dc)
		if err != nil {
			return TiltfileLoadResult{}, err
		}
	}

	err = s.checkForUnconsumedLiveUpdateSteps()
	if err != nil {
		return TiltfileLoadResult{}, err
	}

	s.checkForFastBuilds(manifests)

	manifests, err = match(manifests, matching)
	if err != nil {
		return TiltfileLoadResult{}, err
	}

	yamlManifest := model.Manifest{}
	if len(unresourced) > 0 {
		yamlManifest, err = k8s.NewK8sOnlyManifest(model.UnresourcedYAMLManifestName, unresourced)
		if err != nil {
			return TiltfileLoadResult{}, err
		}
		manifests = append(manifests, yamlManifest)
	}

	printWarnings(s)
	printedWarnings = true

	s.logger.Infof("Successfully loaded Tiltfile")

	tfl.reportTiltfileLoaded(s.builtinCallCounts)

	tiltIgnoreContents, err := s.readFile(tiltIgnorePath(absFilename))
	// missing tiltignore is fine
	if os.IsNotExist(err) {
		err = nil
	} else if err != nil {
		return TiltfileLoadResult{}, errors.Wrapf(err, "error reading %s", tiltIgnorePath(absFilename))
	}

	return TiltfileLoadResult{
		Manifests:          manifests,
		ConfigFiles:        s.configFiles,
		Warnings:           s.warnings,
		TiltIgnoreContents: string(tiltIgnoreContents),
		FeatureFlags:       s.features.ToEnabled(),
		TeamName:           s.teamName,
	}, err
}

// .tiltignore sits next to Tiltfile
func tiltIgnorePath(tiltfilePath string) string {
	return filepath.Join(filepath.Dir(tiltfilePath), TiltIgnoreFileName)
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
	if seq, ok := v.(starlark.Sequence); ok {
		var ret []starlark.Value
		it := seq.Iterate()
		defer it.Done()
		var i starlark.Value
		for it.Next(&i) {
			ret = append(ret, i)
		}
		return ret
	} else if v == nil || v == starlark.None {
		return nil
	} else {
		return []starlark.Value{v}
	}
}

func (tfl *tiltfileLoader) reportTiltfileLoaded(counts map[string]int) {
	tags := make(map[string]string)
	for builtinName, count := range counts {
		tags[fmt.Sprintf("tiltfile.invoked.%s", builtinName)] = strconv.Itoa(count)
	}
	tfl.analytics.Incr("tiltfile.loaded", tags)
}
