package tiltfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	wmanalytics "github.com/tilt-dev/wmclient/pkg/analytics"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/controllers/apiset"
	"github.com/tilt-dev/tilt/internal/tiltfile/tiltextension"
	"github.com/tilt-dev/tilt/internal/tiltfile/v1alpha1"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	tiltfileanalytics "github.com/tilt-dev/tilt/internal/tiltfile/analytics"
	"github.com/tilt-dev/tilt/internal/tiltfile/config"
	"github.com/tilt-dev/tilt/internal/tiltfile/dockerprune"
	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/k8scontext"
	"github.com/tilt-dev/tilt/internal/tiltfile/metrics"
	"github.com/tilt-dev/tilt/internal/tiltfile/secretsettings"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/telemetry"
	"github.com/tilt-dev/tilt/internal/tiltfile/updatesettings"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/internal/tiltfile/version"
	"github.com/tilt-dev/tilt/internal/tiltfile/watch"
	"github.com/tilt-dev/tilt/pkg/model"
)

const FileName = "Tiltfile"

type TiltfileLoadResult struct {
	Manifests           []model.Manifest
	Tiltignore          model.Dockerignore
	ConfigFiles         []string
	FeatureFlags        map[string]bool
	TeamID              string
	TelemetrySettings   model.TelemetrySettings
	MetricsSettings     model.MetricsSettings
	Secrets             model.SecretSet
	Error               error
	DockerPruneSettings model.DockerPruneSettings
	AnalyticsOpt        wmanalytics.Opt
	VersionSettings     model.VersionSettings
	UpdateSettings      model.UpdateSettings
	WatchSettings       model.WatchSettings
	ObjectSet           apiset.ObjectSet

	// For diagnostic purposes only
	BuiltinCalls []starkit.BuiltinCall `json:"-"`
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

func ProvideTiltfileLoader(
	analytics *analytics.TiltAnalytics,
	kCli k8s.Client,
	k8sContextExt k8scontext.Plugin,
	versionExt version.Plugin,
	configExt *config.Plugin,
	dcCli dockercompose.DockerComposeClient,
	webHost model.WebHost,
	localEnv *localexec.Env,
	fDefaults feature.Defaults,
	env k8s.Env) TiltfileLoader {
	return tiltfileLoader{
		analytics:     analytics,
		kCli:          kCli,
		k8sContextExt: k8sContextExt,
		versionExt:    versionExt,
		configExt:     configExt,
		dcCli:         dcCli,
		webHost:       webHost,
		localEnv:      localEnv,
		fDefaults:     fDefaults,
		env:           env,
	}
}

type tiltfileLoader struct {
	analytics *analytics.TiltAnalytics
	kCli      k8s.Client
	dcCli     dockercompose.DockerComposeClient
	webHost   model.WebHost
	localEnv  *localexec.Env

	k8sContextExt k8scontext.Plugin
	versionExt    version.Plugin
	configExt     *config.Plugin
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

	tiltignorePath := watch.TiltignorePath(absFilename)
	tlr := TiltfileLoadResult{
		ConfigFiles: []string{absFilename, tiltignorePath},
	}

	tiltignore, err := watch.ReadTiltignore(tiltignorePath)

	// missing tiltignore is fine, but a filesystem error is not
	if err != nil {
		tlr.Error = err
		return tlr
	}

	tlr.Tiltignore = tiltignore

	localRegistry := tfl.kCli.LocalRegistry(ctx)

	s := newTiltfileState(ctx, tfl.dcCli, tfl.webHost, tfl.localEnv, tfl.k8sContextExt, tfl.versionExt, tfl.configExt, localRegistry, feature.FromDefaults(tfl.fDefaults))

	manifests, result, err := s.loadManifests(absFilename, userConfigState)

	tlr.BuiltinCalls = result.BuiltinCalls

	// All data models are loaded with GetState. We ignore the error if the state
	// isn't properly loaded. This is necessary for handling partial Tiltfile
	// execution correctly, where some state is correctly assembled but other
	// state is not (and should be assumed empty).
	ws, _ := watch.GetState(result)
	tlr.WatchSettings = ws

	// NOTE(maia): if/when add secret settings that affect the engine, add them to tlr here
	ss, _ := secretsettings.GetState(result)
	s.secretSettings = ss

	ioState, _ := io.GetState(result)

	tlr.ConfigFiles = append(tlr.ConfigFiles, ioState.Paths...)
	tlr.ConfigFiles = append(tlr.ConfigFiles, s.postExecReadFiles...)
	tlr.ConfigFiles = sliceutils.DedupedAndSorted(tlr.ConfigFiles)

	dps, _ := dockerprune.GetState(result)
	tlr.DockerPruneSettings = dps

	aSettings, _ := tiltfileanalytics.GetState(result)
	tlr.AnalyticsOpt = aSettings.Opt

	tlr.Secrets = s.extractSecrets()
	tlr.FeatureFlags = s.features.ToEnabled()
	tlr.Error = err
	tlr.Manifests = manifests
	tlr.TeamID = s.teamID

	objectSet, _ := v1alpha1.GetState(result)
	tlr.ObjectSet = objectSet

	vs, _ := version.GetState(result)
	tlr.VersionSettings = vs

	telemetrySettings, _ := telemetry.GetState(result)
	tlr.TelemetrySettings = telemetrySettings

	metricsSettings, _ := metrics.GetState(result)
	tlr.MetricsSettings = metricsSettings

	us, _ := updatesettings.GetState(result)
	tlr.UpdateSettings = us

	duration := time.Since(start)
	if tlr.Error == nil {
		s.logger.Infof("Successfully loaded Tiltfile (%s)", duration)
	}
	extState, _ := tiltextension.GetState(result)
	tfl.reportTiltfileLoaded(s.builtinCallCounts, s.builtinArgCounts, duration, extState.ExtsLoaded)
	reportTiltfileExecMetrics(ctx, duration, err != nil)

	if len(aSettings.CustomTagsToReport) > 0 {
		reportCustomTags(tfl.analytics, aSettings.CustomTagsToReport)
	}

	return tlr
}

func starlarkValueOrSequenceToSlice(v starlark.Value) []starlark.Value {
	return value.ValueOrSequenceToSlice(v)
}

func reportCustomTags(a *analytics.TiltAnalytics, tags map[string]string) {
	a.Incr("tiltfile.custom.report", tags)
}

func (tfl *tiltfileLoader) reportTiltfileLoaded(
	callCounts map[string]int, argCounts map[string]map[string]int,
	loadDur time.Duration, pluginsLoaded map[string]bool) {
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
	for ext := range pluginsLoaded {
		tags := map[string]string{
			"env":      string(tfl.env),
			"ext_name": ext,
		}
		tfl.analytics.Incr("tiltfile.loaded.plugin", tags)
	}
}
