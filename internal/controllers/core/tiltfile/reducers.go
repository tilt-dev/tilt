package tiltfile

import (
	"context"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const TiltfileBuildSource = "tiltfile"

func HandleConfigsReloadStarted(
	ctx context.Context,
	state *store.EngineState,
	event ConfigsReloadStartedAction,
) {
	ms, ok := state.TiltfileStates[event.Name]
	if !ok {
		return
	}

	status := model.BuildRecord{
		StartTime: event.StartTime,
		Reason:    event.Reason,
		Edits:     event.FilesChanged,
		SpanID:    event.SpanID,
	}
	ms.CurrentBuilds[TiltfileBuildSource] = status
	state.RemoveFromTriggerQueue(event.Name)
}

// In the original Tilt architecture, the Tiltfile contained
// the whole engine state. Reloading the tiltfile re-created that
// state from scratch.
//
// We've moved towards a more modular architecture, but many of the tilt data
// models aren't modular. For example, if two Tiltfiles set UpdateSettings,
// it's not clear which one "wins" or how their preferences combine.
//
// In the long-term, Tiltfile settings should only take affect in objects created
// by that Tiltfile. (e.g., WatchSettings only affects FileWatches created by
// that Tiltfile.)
//
// In the medium-term, we resolve this in the EngineState in three different ways:
//  1. If a data structure supports merging (like the Manifest map), do a merge.
//  2. If merging fails (like if two Tiltfiles define the same Manifest), log an Error
//     and try to do something reasonable.
//  3. If a data structure does not support merging (like UpdateSettings), only
//     accept that data structure from the "main" tiltfile.
func HandleConfigsReloaded(
	ctx context.Context,
	state *store.EngineState,
	event ConfigsReloadedAction,
) {
	isMainTiltfile := event.Name == model.MainTiltfileManifestName

	manifests := event.Manifests
	loadedManifestNames := map[model.ManifestName]bool{}
	for i, m := range manifests {
		loadedManifestNames[m.Name] = true

		// Properly annotate the manifest with the source tiltfile.
		m.SourceTiltfile = event.Name
		manifests[i] = m
	}

	ms, ok := state.TiltfileStates[event.Name]
	if !ok {
		return
	}
	b := ms.CurrentBuilds[TiltfileBuildSource]

	// Remove pending file changes that were consumed by this build.
	for _, status := range ms.BuildStatuses {
		status.ConsumeChangesBefore(b.StartTime)
	}

	// Track the new secrets and go back to scrub them.
	newSecrets := model.SecretSet{}
	for k, v := range event.Secrets {
		_, exists := state.Secrets[k]
		if !exists {
			newSecrets[k] = v
		}
	}

	// Add all secrets, even if we failed.
	state.Secrets.AddAll(event.Secrets)

	// Retroactively scrub secrets
	state.LogStore.ScrubSecretsStartingAt(newSecrets, event.CheckpointAtExecStart)

	// Add team id if it exists, even if execution failed.
	if isMainTiltfile && (event.TeamID != "" || event.Err == nil) {
		state.TeamID = event.TeamID
	}

	// if the ConfigsReloadedAction came from a unit test, there might not be a current build
	if !b.Empty() {
		b.FinishTime = event.FinishTime
		b.Error = event.Err

		if b.SpanID != "" {
			b.WarningCount = len(state.LogStore.Warnings(b.SpanID))
		}

		ms.AddCompletedBuild(b)
	}
	delete(ms.CurrentBuilds, TiltfileBuildSource)
	if event.Err != nil {
		// When the Tiltfile had an error, we want to differentiate between two cases:
		//
		// 1) You're running `tilt up` for the first time, and a local() command
		// exited with status code 1.  Partial results (like enabling features)
		// would be helpful.
		//
		// 2) You're running 'tilt up' in the happy state. You edit the Tiltfile,
		// and introduce a syntax error.  You don't want partial results to wipe out
		// your "good" state.

		if isMainTiltfile {
			// Enable any new features in the partial state.
			if len(state.Features) == 0 {
				state.Features = event.Features
			} else {
				for feature, val := range event.Features {
					if val {
						state.Features[feature] = val
					}
				}
			}
		}
		return
	}

	// Make sure all the new manifests are in the EngineState.
	for _, m := range manifests {
		mt, ok := state.ManifestTargets[m.ManifestName()]
		if ok && mt.Manifest.SourceTiltfile != event.Name {
			logger.Get(ctx).Errorf("Resource defined in two tiltfiles: %s, %s", event.Name, mt.Manifest.SourceTiltfile)
			continue
		}

		// Create a new manifest if it changed types.
		createNew := !ok ||
			mt.Manifest.IsK8s() != m.IsK8s() ||
			mt.Manifest.IsLocal() != m.IsLocal() ||
			mt.Manifest.IsDC() != m.IsDC()
		if createNew {
			mt = store.NewManifestTarget(m)
		}

		configFilesThatChanged := ms.LastBuild().Edits
		old := mt.Manifest
		mt.Manifest = m

		if model.ChangesInvalidateBuild(old, m) {
			// Manifest has changed such that the current build is invalid;
			// ensure we do an image build so that we apply the changes
			ms := mt.State
			ms.BuildStatuses = make(map[model.TargetID]*store.BuildStatus)
			ms.PendingManifestChange = event.FinishTime
			ms.ConfigFilesThatCausedChange = configFilesThatChanged
		}
		state.UpsertManifestTarget(mt)
	}

	// Go through all the existing manifest targets. If they were from this
	// Tiltfile, but were removed from the latest Tiltfile execution, delete them.
	for _, mt := range state.Targets() {
		m := mt.Manifest

		if m.SourceTiltfile == event.Name {
			if !loadedManifestNames[m.Name] {
				state.RemoveManifestTarget(m.Name)
			}
			continue
		}
	}

	// Global state that's only configurable from the main manifest.
	if isMainTiltfile {
		state.Features = event.Features
		state.TelemetrySettings = event.TelemetrySettings
		state.VersionSettings = event.VersionSettings
		state.AnalyticsTiltfileOpt = event.AnalyticsTiltfileOpt
		state.UpdateSettings = event.UpdateSettings
		state.DockerPruneSettings = event.DockerPruneSettings
	}
}
