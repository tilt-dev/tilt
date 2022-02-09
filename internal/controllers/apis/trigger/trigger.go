package trigger

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var fwGVK = v1alpha1.SchemeGroupVersion.WithKind("FileWatch")
var btnGVK = v1alpha1.SchemeGroupVersion.WithKind("UIButton")

var triggerTypes = []client.Object{
	&v1alpha1.FileWatch{},
	&v1alpha1.UIButton{},
}

type ExtractFunc func(obj client.Object) TriggerSpecs

// Objects is a container for objects referenced by TriggerSpecs
type Objects struct {
	UIButtons   map[string]*v1alpha1.UIButton
	FileWatches map[string]*v1alpha1.FileWatch
}

type TriggerSpecs struct {
	RestartOn *v1alpha1.RestartOnSpec
	StartOn   *v1alpha1.StartOnSpec
	StopOn    *v1alpha1.StopOnSpec
}

// SetupController creates watches for types referenced by the given specs and registers
// an index function for them.
func SetupController(builder *builder.Builder, idxer *indexer.Indexer, extractFunc ExtractFunc) {
	idxer.AddKeyFunc(
		func(obj client.Object) []indexer.Key {
			specs := extractFunc(obj)
			return extractKeysForIndexer(obj.GetNamespace(), specs)
		})

	registerWatches(builder, idxer)
}

// FetchObjects retrieves all objects referenced in TriggerSpecs
func FetchObjects(ctx context.Context, client client.Reader, specs TriggerSpecs) (Objects, error) {
	buttons, err := Buttons(ctx, client, specs)
	if err != nil {
		return Objects{}, err
	}

	fileWatches, err := FileWatches(ctx, client, specs.RestartOn)
	if err != nil {
		return Objects{}, err
	}

	return Objects{
		UIButtons:   buttons,
		FileWatches: fileWatches,
	}, nil
}

// Fetch all the buttons that this object depends on.
//
// If a button isn't in the API server yet, it will simply be missing from the map.
//
// Other errors reaching the API server will be returned to the caller.
//
// TODO(nick): If the user typos a button name, there's currently no feedback
// that this is happening. This is probably the correct product behavior (in particular:
// resources should still run if their restarton button has been deleted).
// We might eventually need some sort of StartOnStatus/RestartOnStatus to express errors
// in lookup.
func Buttons(ctx context.Context, client client.Reader, specs TriggerSpecs) (map[string]*v1alpha1.UIButton, error) {
	buttonNames := []string{}
	if specs.StartOn != nil {
		buttonNames = append(buttonNames, specs.StartOn.UIButtons...)
	}

	if specs.RestartOn != nil {
		buttonNames = append(buttonNames, specs.RestartOn.UIButtons...)
	}

	if specs.StopOn != nil {
		buttonNames = append(buttonNames, specs.StopOn.UIButtons...)
	}

	result := make(map[string]*v1alpha1.UIButton, len(buttonNames))
	for _, n := range buttonNames {
		_, exists := result[n]
		if exists {
			continue
		}

		b := &v1alpha1.UIButton{}
		err := client.Get(ctx, types.NamespacedName{Name: n}, b)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		result[n] = b
	}
	return result, nil
}

// Fetch all the filewatches that this object depends on.
//
// If a filewatch isn't in the API server yet, it will simply be missing from the map.
//
// Other errors reaching the API server will be returned to the caller.
//
// TODO(nick): If the user typos a filewatch name, there's currently no feedback
// that this is happening. This is probably the correct product behavior (in particular:
// resources should still run if their restarton filewatch has been deleted).
// We might eventually need some sort of RestartOnStatus to express errors
// in lookup.
func FileWatches(ctx context.Context, client client.Reader, restartOn *v1alpha1.RestartOnSpec) (map[string]*v1alpha1.FileWatch, error) {
	if restartOn == nil {
		return nil, nil
	}

	result := make(map[string]*v1alpha1.FileWatch, len(restartOn.FileWatches))
	for _, n := range restartOn.FileWatches {
		fw := &v1alpha1.FileWatch{}
		err := client.Get(ctx, types.NamespacedName{Name: n}, fw)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		result[n] = fw
	}
	return result, nil
}

// Fetch the last time a start was requested from this target's dependencies.
//
// Returns the most recent trigger time. If the most recent trigger is a button,
// return the button. Some consumers use the button for text inputs.
func LastStartEvent(startOn *v1alpha1.StartOnSpec, triggerObjs Objects) (time.Time, *v1alpha1.UIButton) {
	latestTime := time.Time{}
	var latestButton *v1alpha1.UIButton
	if startOn == nil {
		return time.Time{}, nil
	}

	for _, bn := range startOn.UIButtons {
		b, ok := triggerObjs.UIButtons[bn]
		if !ok {
			// ignore missing buttons
			continue
		}
		lastEventTime := b.Status.LastClickedAt
		if !lastEventTime.Time.Before(startOn.StartAfter.Time) && lastEventTime.Time.After(latestTime) {
			latestTime = lastEventTime.Time
			latestButton = b
		}
	}

	return latestTime, latestButton
}

// Fetch the last time a restart was requested from this target's dependencies.
//
// Returns the most recent trigger time. If the most recent trigger is a button,
// return the button. Some consumers use the button for text inputs.
func LastRestartEvent(restartOn *v1alpha1.RestartOnSpec, triggerObjs Objects) (time.Time, *v1alpha1.UIButton) {
	cur := time.Time{}
	var latestButton *v1alpha1.UIButton
	if restartOn == nil {
		return cur, nil
	}

	for _, fwn := range restartOn.FileWatches {
		fw, ok := triggerObjs.FileWatches[fwn]
		if !ok {
			// ignore missing filewatches
			continue
		}
		lastEventTime := fw.Status.LastEventTime
		if lastEventTime.Time.After(cur) {
			cur = lastEventTime.Time
		}
	}

	for _, bn := range restartOn.UIButtons {
		b, ok := triggerObjs.UIButtons[bn]
		if !ok {
			// ignore missing buttons
			continue
		}
		lastEventTime := b.Status.LastClickedAt
		if lastEventTime.Time.After(cur) {
			cur = lastEventTime.Time
			latestButton = b
		}
	}

	return cur, latestButton
}

// Fetch the set of files that have changed since the given timestamp.
// We err on the side of undercounting (i.e., skipping files that may have triggered
// this build but are not sure).
func FilesChanged(restartOn *v1alpha1.RestartOnSpec, fileWatches map[string]*v1alpha1.FileWatch, lastBuild time.Time) []string {
	filesChanged := []string{}
	if restartOn == nil {
		return filesChanged
	}
	for _, fwn := range restartOn.FileWatches {
		fw, ok := fileWatches[fwn]
		if !ok {
			// ignore missing filewatches
			continue
		}

		// Add files so that the most recent files are first.
		for i := len(fw.Status.FileEvents) - 1; i >= 0; i-- {
			e := fw.Status.FileEvents[i]
			if e.Time.Time.After(lastBuild) {
				filesChanged = append(filesChanged, e.SeenFiles...)
			}
		}
	}
	return sliceutils.DedupedAndSorted(filesChanged)
}

// registerWatches ensures that reconciliation happens on changes to objects referenced by TriggerSpecs
func registerWatches(builder *builder.Builder, indexer *indexer.Indexer) {
	for _, t := range triggerTypes {
		// this is arguably overly defensive, but a copy of the type object stub is made
		// to avoid sharing references of it across different reconcilers
		obj := t.DeepCopyObject().(client.Object)
		builder.Watches(&source.Kind{Type: obj},
			handler.EnqueueRequestsFromMapFunc(indexer.Enqueue))
	}
}

// extractKeysForIndexer returns the keys of objects referenced in the RestartOnSpec and/or StartOnSpec.
func extractKeysForIndexer(
	namespace string,
	specs TriggerSpecs,
) []indexer.Key {
	var keys []indexer.Key

	if specs.RestartOn != nil {
		for _, name := range specs.RestartOn.FileWatches {
			keys = append(keys, indexer.Key{
				Name: types.NamespacedName{Namespace: namespace, Name: name},
				GVK:  fwGVK,
			})
		}

		for _, name := range specs.RestartOn.UIButtons {
			keys = append(keys, indexer.Key{
				Name: types.NamespacedName{Namespace: namespace, Name: name},
				GVK:  btnGVK,
			})
		}
	}

	if specs.StartOn != nil {
		for _, name := range specs.StartOn.UIButtons {
			keys = append(keys, indexer.Key{
				Name: types.NamespacedName{Namespace: namespace, Name: name},
				GVK:  btnGVK,
			})
		}
	}

	if specs.StopOn != nil {
		for _, name := range specs.StopOn.UIButtons {
			keys = append(keys, indexer.Key{
				Name: types.NamespacedName{Namespace: namespace, Name: name},
				GVK:  btnGVK,
			})
		}
	}

	return keys
}

// Fetch the last time a start was requested from this target's dependencies.
//
// Returns the most recent trigger time. If the most recent trigger is a button,
// return the button. Some consumers use the button for text inputs.
func LastStopEvent(stopOn *v1alpha1.StopOnSpec, restartObjs Objects) (time.Time, *v1alpha1.UIButton) {
	latestTime := time.Time{}
	var latestButton *v1alpha1.UIButton
	if stopOn == nil {
		return time.Time{}, nil
	}

	for _, bn := range stopOn.UIButtons {
		b, ok := restartObjs.UIButtons[bn]
		if !ok {
			// ignore missing buttons
			continue
		}
		lastEventTime := b.Status.LastClickedAt
		if lastEventTime.Time.After(latestTime) {
			latestTime = lastEventTime.Time
			latestButton = b
		}
	}

	return latestTime, latestButton
}
