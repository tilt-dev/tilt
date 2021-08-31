package restarton

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

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
func Buttons(ctx context.Context, client client.Client, restartOn *v1alpha1.RestartOnSpec, startOn *v1alpha1.StartOnSpec) (map[string]*v1alpha1.UIButton, error) {
	buttonNames := []string{}
	if startOn != nil {
		buttonNames = append(buttonNames, startOn.UIButtons...)
	}

	if restartOn != nil {
		buttonNames = append(buttonNames, restartOn.UIButtons...)
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
func FileWatches(ctx context.Context, client client.Client, restartOn *v1alpha1.RestartOnSpec) (map[string]*v1alpha1.FileWatch, error) {
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
func LastStartEvent(startOn *v1alpha1.StartOnSpec, buttons map[string]*v1alpha1.UIButton) (time.Time, *v1alpha1.UIButton) {
	latestTime := time.Time{}
	var latestButton *v1alpha1.UIButton
	if startOn == nil {
		return time.Time{}, nil
	}

	for _, bn := range startOn.UIButtons {
		b, ok := buttons[bn]
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
func LastRestartEvent(restartOn *v1alpha1.RestartOnSpec, fileWatches map[string]*v1alpha1.FileWatch, buttons map[string]*v1alpha1.UIButton) (time.Time, *v1alpha1.UIButton) {
	cur := time.Time{}
	var latestButton *v1alpha1.UIButton
	if restartOn == nil {
		return cur, nil
	}

	for _, fwn := range restartOn.FileWatches {
		fw, ok := fileWatches[fwn]
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
		b, ok := buttons[bn]
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
