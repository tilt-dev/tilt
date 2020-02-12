package tiltfile

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	wmanalytics "github.com/windmilleng/wmclient/pkg/analytics"
	"go.starlark.net/resolve"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/updatesettings"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/sliceutils"
	tiltfileanalytics "github.com/windmilleng/tilt/internal/tiltfile/analytics"
	"github.com/windmilleng/tilt/internal/tiltfile/dockerprune"
	"github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/k8scontext"
	"github.com/windmilleng/tilt/internal/tiltfile/telemetry"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
	"github.com/windmilleng/tilt/internal/tiltfile/version"
	"github.com/windmilleng/tilt/pkg/model"
)

const FileName = "Tiltfile"
const TiltIgnoreFileName = ".tiltignore"

func init() {
	resolve.AllowLambda = true
	resolve.AllowNestedDef = true
	resolve.AllowGlobalReassign = true
	resolve.AllowRecursion = true
}

type TiltfileLoadResult struct {
	Manifests           []model.Manifest
	ConfigFiles         []string
	TiltIgnoreContents  string
	FeatureFlags        map[string]bool
	TeamName            string
	TelemetrySettings   model.TelemetrySettings
	Secrets             model.SecretSet
	Error               error
	DockerPruneSettings model.DockerPruneSettings
	AnalyticsOpt        wmanalytics.Opt
	VersionSettings     model.VersionSettings
	UpdateSettings      model.UpdateSettings
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
	Load(ctx context.Context, filename string, userConfigState model.UserConfigState) TiltfileLoadResult
}

type FakeTiltfileLoader struct {
	Result          TiltfileLoadResult
	userConfigState model.UserConfigState
}

var _ TiltfileLoader = &FakeTiltfileLoader{}

func NewFakeTiltfileLoader() *FakeTiltfileLoader {
	return &FakeTiltfileLoader{}
}

func (tfl *FakeTiltfileLoader) Load(ctx context.Context, filename string, userConfigState model.UserConfigState) TiltfileLoadResult {
	tfl.userConfigState = userConfigState
	return tfl.Result
}

// the UserConfigState that was passed to the last invocation of Load
func (tfl *FakeTiltfileLoader) PassedUserConfigState() model.UserConfigState {
	return tfl.userConfigState
}

func ProvideTiltfileLoader(
	analytics *analytics.TiltAnalytics,
	kCli k8s.Client,
	k8sContextExt k8scontext.Extension,
	dcCli dockercompose.DockerComposeClient,
	webHost model.WebHost,
	fDefaults feature.Defaults,
	env k8s.Env) TiltfileLoader {
	return tiltfileLoader{
		analytics:     analytics,
		kCli:          kCli,
		k8sContextExt: k8sContextExt,
		dcCli:         dcCli,
		webHost:       webHost,
		fDefaults:     fDefaults,
		env:           env,
	}
}

type tiltfileLoader struct {
	analytics *analytics.TiltAnalytics
	kCli      k8s.Client
	dcCli     dockercompose.DockerComposeClient
	webHost   model.WebHost

	k8sContextExt k8scontext.Extension
	fDefaults     feature.Defaults
	env           k8s.Env
}

var _ TiltfileLoader = &tiltfileLoader{}

// Load loads the Tiltfile in `filename`
func (tfl tiltfileLoader) Load(ctx context.Context, filename string, userConfigState model.UserConfigState) TiltfileLoadResult {
	start := time.Now()
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

	localRegistry := tfl.kCli.LocalRegistry(ctx)

	s := newTiltfileState(ctx, tfl.dcCli, tfl.webHost, tfl.k8sContextExt, localRegistry, feature.FromDefaults(tfl.fDefaults))

	manifests, result, err := s.loadManifests(absFilename, userConfigState)

	ioState, _ := io.GetState(result)
	tlr.ConfigFiles = sliceutils.AppendWithoutDupes(ioState.Files, s.postExecReadFiles...)

	dps, _ := dockerprune.GetState(result)
	tlr.DockerPruneSettings = dps

	aSettings, _ := tiltfileanalytics.GetState(result)
	tlr.AnalyticsOpt = aSettings.Opt

	tlr.Secrets = s.extractSecrets()
	tlr.FeatureFlags = s.features.ToEnabled()
	tlr.Error = err
	tlr.Manifests = manifests
	tlr.TeamName = s.teamName

	vs, _ := version.GetState(result)
	tlr.VersionSettings = vs

	telemetrySettings, _ := telemetry.GetState(result)
	tlr.TelemetrySettings = telemetrySettings

	us, _ := updatesettings.GetState(result)
	tlr.UpdateSettings = us

	duration := time.Since(start)
	s.logger.Infof("Successfully loaded Tiltfile (%s)", duration)
	tfl.reportTiltfileLoaded(s.builtinCallCounts, s.builtinArgCounts, duration)

	return tlr
}

// .tiltignore sits next to Tiltfile
func tiltIgnorePath(tiltfilePath string) string {
	return filepath.Join(filepath.Dir(tiltfilePath), TiltIgnoreFileName)
}

func starlarkValueOrSequenceToSlice(v starlark.Value) []starlark.Value {
	return value.ValueOrSequenceToSlice(v)
}

func (tfl *tiltfileLoader) reportTiltfileLoaded(callCounts map[string]int,
	argCounts map[string]map[string]int, loadDur time.Duration) {
	tags := make(map[string]string)

	// env should really be a global tag, but there's a circular dependency
	// between the global tags and env initialization, so we add it manually.
	tags["env"] = string(tfl.env)

	for builtinName, count := range callCounts {
		tags[fmt.Sprintf("tiltfile.invoked.%s", builtinName)] = strconv.Itoa(count)
	}
	for builtinName, counts := range argCounts {
		for argName, count := range counts {
			tags[fmt.Sprintf("tiltfile.invoked.%s.arg.%s", builtinName, argName)] = strconv.Itoa(count)
		}
	}
	tfl.analytics.Incr("tiltfile.loaded", tags)
	tfl.analytics.Timer("tiltfile.load", loadDur, nil)
}
