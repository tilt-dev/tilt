package trigger

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var fwGVK = v1alpha1.SchemeGroupVersion.WithKind("FileWatch")
var btnGVK = v1alpha1.SchemeGroupVersion.WithKind("UIButton")

// SetupControllerRestartOn sets up watchers / indexers for a type with a RestartOnSpec
func SetupControllerRestartOn(builder *builder.Builder, idxer *indexer.Indexer, extract func(obj client.Object) *v1alpha1.RestartOnSpec) {
	idxer.AddKeyFunc(
		func(obj client.Object) []indexer.Key {
			spec := extract(obj)
			if spec == nil {
				return nil
			}
			var keys []indexer.Key
			keys = append(keys, indexerKeys(fwGVK, obj.GetNamespace(), spec.FileWatches)...)
			keys = append(keys, indexerKeys(btnGVK, obj.GetNamespace(), spec.UIButtons)...)
			return keys
		})

	registerWatches(builder, idxer, []client.Object{&v1alpha1.FileWatch{}, &v1alpha1.UIButton{}})
}

// SetupControllerStartOn sets up watchers / indexers for a type with a StartOnSpec
func SetupControllerStartOn(builder *builder.Builder, idxer *indexer.Indexer, extract func(obj client.Object) *v1alpha1.StartOnSpec) {
	idxer.AddKeyFunc(
		func(obj client.Object) []indexer.Key {
			spec := extract(obj)
			if spec == nil {
				return nil
			}
			return indexerKeys(btnGVK, obj.GetNamespace(), spec.UIButtons)
		})

	registerWatches(builder, idxer, []client.Object{&v1alpha1.UIButton{}})
}

// SetupControllerStopOn sets up watchers / indexers for a type with a StopOnSpec
func SetupControllerStopOn(builder *builder.Builder, idxer *indexer.Indexer, extract func(obj client.Object) *v1alpha1.StopOnSpec) {
	idxer.AddKeyFunc(
		func(obj client.Object) []indexer.Key {
			spec := extract(obj)
			if spec == nil {
				return nil
			}
			return indexerKeys(btnGVK, obj.GetNamespace(), spec.UIButtons)
		})

	registerWatches(builder, idxer, []client.Object{&v1alpha1.UIButton{}})
}

func indexerKeys(gvk schema.GroupVersionKind, namespace string, names []string) []indexer.Key {
	var keys []indexer.Key
	for _, name := range names {
		keys = append(keys, indexer.Key{
			Name: types.NamespacedName{Namespace: namespace, Name: name},
			GVK:  gvk,
		})
	}
	return keys
}

func registerWatches(builder *builder.Builder, idxer *indexer.Indexer, typesToWatch []client.Object) {
	for _, t := range typesToWatch {
		builder.Watches(t, handler.EnqueueRequestsFromMapFunc(idxer.Enqueue))
	}
}

// fetchButtons retrieves all the buttons that this object depends on.
//
// If a button isn't in the API server yet, it will simply be missing from the map.
//
// Other errors reaching the API server will be returned to the caller.
//
// TODO(nick): If the user typos a button name, there's currently no feedback
//
//	that this is happening. This is probably the correct product behavior (in particular:
//	resources should still run if one of their triggers has been deleted).
//	We might eventually need trigger statuses to express errors in lookup.
func fetchButtons(ctx context.Context, client client.Reader, buttonNames []string) (map[string]*v1alpha1.UIButton, error) {
	buttons := make(map[string]*v1alpha1.UIButton, len(buttonNames))
	for _, n := range buttonNames {
		_, exists := buttons[n]
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
		buttons[n] = b
	}
	return buttons, nil
}

// fetchFileWatches retrieves all the filewatches that this object depends on.
//
// If a filewatch isn't in the API server yet, it will simply be missing from the slice.
//
// Other errors reaching the API server will be returned to the caller.
//
// TODO(nick): If the user typos a filewatch name, there's currently no feedback
//
//	that this is happening. This is probably the correct product behavior (in particular:
//	resources should still run if one of their triggers has been deleted).
//	We might eventually need trigger statuses to express errors in lookup.
func fetchFileWatches(ctx context.Context, client client.Reader, fwNames []string) ([]*v1alpha1.FileWatch, error) {
	result := []*v1alpha1.FileWatch{}
	for _, n := range fwNames {
		fw := &v1alpha1.FileWatch{}
		err := client.Get(ctx, types.NamespacedName{Name: n}, fw)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		result = append(result, fw)
	}
	return result, nil
}

// LastStartEvent determines the last time a start was requested from this target's dependencies.
//
// Returns the most recent start time. If the most recent start is from a button,
// return the button. Some consumers use the button for text inputs.
func LastStartEvent(ctx context.Context, cli client.Reader, startOn *v1alpha1.StartOnSpec) (metav1.MicroTime, *v1alpha1.UIButton, error) {
	if startOn == nil {
		return metav1.MicroTime{}, nil, nil
	}

	buttons, err := fetchButtons(ctx, cli, startOn.UIButtons)
	if err != nil {
		return metav1.MicroTime{}, nil, err
	}

	var latestTime metav1.MicroTime
	var latestButton *v1alpha1.UIButton

	// ensure predictable iteration order by using the list from the spec
	// (currently, missing buttons are simply ignored)
	for _, buttonName := range startOn.UIButtons {
		b := buttons[buttonName]
		if b == nil {
			continue
		}

		lastEventTime := b.Status.LastClickedAt
		if timecmp.AfterOrEqual(lastEventTime, startOn.StartAfter) && timecmp.After(lastEventTime, latestTime) {
			latestTime = lastEventTime
			latestButton = b
		}
	}

	return latestTime, latestButton, nil
}

// LastRestartEvent determines the last time a restart was requested from this target's dependencies.
//
// Returns the most recent restart time.
//
// If the most recent restart is from a button, return the button. Some consumers use the button for text inputs.
// If the most recent restart is from a FileWatch, return the FileWatch. Some consumers use the FileWatch to
// determine if they should re-run or not to avoid repeated failures.
func LastRestartEvent(ctx context.Context, cli client.Reader, restartOn *v1alpha1.RestartOnSpec) (metav1.MicroTime, *v1alpha1.UIButton, []*v1alpha1.FileWatch, error) {
	var fws []*v1alpha1.FileWatch
	if restartOn == nil {
		return metav1.MicroTime{}, nil, fws, nil
	}
	buttons, err := fetchButtons(ctx, cli, restartOn.UIButtons)
	if err != nil {
		return metav1.MicroTime{}, nil, fws, err
	}
	fws, err = fetchFileWatches(ctx, cli, restartOn.FileWatches)
	if err != nil {
		return metav1.MicroTime{}, nil, fws, err
	}

	var cur metav1.MicroTime
	var latestButton *v1alpha1.UIButton

	for _, fw := range fws {
		lastEventTime := fw.Status.LastEventTime
		if timecmp.After(lastEventTime, cur) {
			cur = lastEventTime
		}
	}

	// ensure predictable iteration order by using the list from the spec
	// (currently, missing buttons are simply ignored)
	for _, buttonName := range restartOn.UIButtons {
		b := buttons[buttonName]
		if b == nil {
			continue
		}

		lastEventTime := b.Status.LastClickedAt
		if timecmp.After(lastEventTime, cur) {
			cur = lastEventTime
			latestButton = b
		}
	}

	return cur, latestButton, fws, nil
}

// FilesChanged determines the set of files that have changed since the given timestamp.
// We err on the side of undercounting (i.e., skipping files that may have triggered
// this build but are not sure).
func FilesChanged(restartOn *v1alpha1.RestartOnSpec, fileWatches []*v1alpha1.FileWatch, lastBuild time.Time) []string {
	filesChanged := []string{}
	if restartOn == nil {
		return filesChanged
	}
	for _, fw := range fileWatches {
		// Add files so that the most recent files are first.
		for i := len(fw.Status.FileEvents) - 1; i >= 0; i-- {
			e := fw.Status.FileEvents[i]
			if timecmp.After(e.Time, lastBuild) {
				filesChanged = append(filesChanged, e.SeenFiles...)
			}
		}
	}
	return sliceutils.DedupedAndSorted(filesChanged)
}

// LastStopEvent determines the latest time a stop was requested from this target's dependencies.
//
// Returns the most recent stop time. If the most recent stop is from a button,
// return the button. Some consumers use the button for text inputs.
func LastStopEvent(ctx context.Context, cli client.Reader, stopOn *v1alpha1.StopOnSpec) (metav1.MicroTime, *v1alpha1.UIButton, error) {
	if stopOn == nil {
		return metav1.MicroTime{}, nil, nil
	}

	buttons, err := fetchButtons(ctx, cli, stopOn.UIButtons)
	if err != nil {
		return metav1.MicroTime{}, nil, err
	}

	var latestTime metav1.MicroTime
	var latestButton *v1alpha1.UIButton

	// ensure predictable iteration order by using the list from the spec
	// (currently, missing buttons are simply ignored)
	for _, buttonName := range stopOn.UIButtons {
		b := buttons[buttonName]
		if b == nil {
			continue
		}

		lastEventTime := b.Status.LastClickedAt
		if timecmp.After(lastEventTime, latestTime) {
			latestTime = lastEventTime
			latestButton = b
		}
	}

	return latestTime, latestButton, nil
}
