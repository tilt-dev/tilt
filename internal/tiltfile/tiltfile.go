package tiltfile

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"go.starlark.net/resolve"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/pkg/model"
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
	Error              error
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
	// Load the Tiltfile.
	//
	// By design, Load() always returns a result.
	// We want to be very careful not to treat non-zero exit codes like an error.
	// Because even if the Tiltfile has errors, we might need to watch files
	// or return partial results (like enabled features).
	Load(ctx context.Context, filename string, matching map[string]bool) TiltfileLoadResult
}

type FakeTiltfileLoader struct {
	Result TiltfileLoadResult
}

var _ TiltfileLoader = &FakeTiltfileLoader{}

func NewFakeTiltfileLoader() *FakeTiltfileLoader {
	return &FakeTiltfileLoader{}
}

func (tfl *FakeTiltfileLoader) Load(ctx context.Context, filename string, matching map[string]bool) TiltfileLoadResult {
	return tfl.Result
}

func ProvideTiltfileLoader(
	analytics *analytics.TiltAnalytics,
	kCli k8s.Client,
	dcCli dockercompose.DockerComposeClient,
	kubeContext k8s.KubeContext,
	kubeEnv k8s.Env,
	fDefaults feature.Defaults) TiltfileLoader {
	return tiltfileLoader{
		analytics:   analytics,
		kCli:        kCli,
		dcCli:       dcCli,
		kubeContext: kubeContext,
		kubeEnv:     kubeEnv,
		fDefaults:   fDefaults,
	}
}

type tiltfileLoader struct {
	analytics   *analytics.TiltAnalytics
	kCli        k8s.Client
	dcCli       dockercompose.DockerComposeClient
	kubeContext k8s.KubeContext
	kubeEnv     k8s.Env
	fDefaults   feature.Defaults
}

var _ TiltfileLoader = &tiltfileLoader{}

func printWarnings(s *tiltfileState) {
	for _, w := range s.warnings {
		s.logger.Infof("WARNING: %s\n", w)
	}
}

// Load loads the Tiltfile in `filename`
func (tfl tiltfileLoader) Load(ctx context.Context, filename string, matching map[string]bool) TiltfileLoadResult {
	absFilename, err := ospath.RealAbs(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return TiltfileLoadResult{
				ConfigFiles: []string{filename},
				Error:       fmt.Errorf("No Tiltfile found at paths '%s'. Check out https://docs.tilt.dev/tutorial.html", filename),
			}
		}
		absFilename, _ = filepath.Abs(filename)
		return TiltfileLoadResult{
			ConfigFiles: []string{absFilename},
			Error:       err,
		}
	}

	tiltIgnorePath := tiltIgnorePath(absFilename)
	tlr := TiltfileLoadResult{
		ConfigFiles: []string{absFilename, tiltIgnorePath},
	}

	tiltIgnoreContents, err := ioutil.ReadFile(tiltIgnorePath)

	// missing tiltignore is fine, but a filesystem error is not
	if err != nil && !os.IsNotExist(err) {
		tlr.Error = err
		return tlr
	}

	tlr.TiltIgnoreContents = string(tiltIgnoreContents)

	privateRegistry := tfl.kCli.PrivateRegistry(ctx)
	s := newTiltfileState(ctx, tfl.dcCli, tfl.kubeContext, tfl.kubeEnv, privateRegistry, feature.FromDefaults(tfl.fDefaults))

	manifests, err := s.loadManifests(absFilename, matching)
	tlr.ConfigFiles = s.configFiles
	tlr.Warnings = s.warnings
	tlr.FeatureFlags = s.features.ToEnabled()
	tlr.Error = err
	tlr.Manifests = manifests
	tlr.TeamName = s.teamName

	printWarnings(s)
	s.logger.Infof("Successfully loaded Tiltfile")
	tfl.reportTiltfileLoaded(s.builtinCallCounts, s.builtinArgCounts)

	return tlr
}

// .tiltignore sits next to Tiltfile
func tiltIgnorePath(tiltfilePath string) string {
	return filepath.Join(filepath.Dir(tiltfilePath), TiltIgnoreFileName)
}

func skylarkStringDictToGoMap(d *starlark.Dict) (map[string]string, error) {
	r := map[string]string{}

	for _, tuple := range d.Items() {
		kV, ok := AsString(tuple[0])
		if !ok {
			return nil, fmt.Errorf("key is not a string: %T (%v)", tuple[0], tuple[0])
		}

		k := string(kV)

		vV, ok := AsString(tuple[1])
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

func (tfl *tiltfileLoader) reportTiltfileLoaded(callCounts map[string]int, argCounts map[string]map[string]int) {
	tags := make(map[string]string)
	for builtinName, count := range callCounts {
		tags[fmt.Sprintf("tiltfile.invoked.%s", builtinName)] = strconv.Itoa(count)
	}
	for builtinName, counts := range argCounts {
		for argName, count := range counts {
			tags[fmt.Sprintf("tiltfile.invoked.%s.arg.%s", builtinName, argName)] = strconv.Itoa(count)
		}
	}
	tfl.analytics.Incr("tiltfile.loaded", tags)
}
