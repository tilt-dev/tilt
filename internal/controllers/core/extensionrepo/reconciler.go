package extensionrepo

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tilt-dev/go-get"
	"github.com/tilt-dev/wmclient/pkg/dirs"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

const tiltModulesRelDir = "tilt_modules"

type Reconciler struct {
	ctrlClient ctrlclient.Client
	dlrPath    string

	fetches            map[types.NamespacedName]time.Time
	backoffs           map[types.NamespacedName]time.Duration
	lastKnownDestPaths map[types.NamespacedName]string
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ExtensionRepo{})

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, dir *dirs.TiltDevDir) (*Reconciler, error) {
	dlrPath, err := dir.Abs(tiltModulesRelDir)
	if err != nil {
		return nil, fmt.Errorf("creating extensionrepo controller: %v", err)
	}
	return &Reconciler{
		ctrlClient:         ctrlClient,
		dlrPath:            dlrPath,
		fetches:            make(map[types.NamespacedName]time.Time),
		backoffs:           make(map[types.NamespacedName]time.Duration),
		lastKnownDestPaths: make(map[types.NamespacedName]string),
	}, nil
}

// Downloads extension repos.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName

	var repo v1alpha1.ExtensionRepo
	err := r.ctrlClient.Get(ctx, nn, &repo)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !repo.ObjectMeta.DeletionTimestamp.IsZero() {
		// If a repo is deleted, delete it on disk.
		//
		// This is simple, but not accurate.
		// 1) It will garbage collect too aggressively if two extension repo
		//    objects point to the same URL.
		// 2) If we "miss" a delete event, the repo will never get cleaned up.
		//
		// A "real" implementation would use the on-disk repos as the source of
		// truth, and garbage collect ones with no remaining refs.
		path, ok := r.lastKnownDestPaths[nn]
		if !ok {
			return ctrl.Result{}, nil
		}
		err := os.RemoveAll(path)
		if err != nil && !os.IsNotExist(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check that the URL is valid.
	importPath, err := getImportPath(&repo)
	if err != nil {
		logger.Get(ctx).Errorf("Invalid ExtensionRepo %s: %v", repo.Name, err)
		return r.updateStatus(ctx, &repo, func(status *v1alpha1.ExtensionRepoStatus) {
			status.Error = err.Error()
		})
	}

	dlr := get.NewDownloader(r.dlrPath)
	destPath := dlr.DestinationPath(importPath)
	info, err := os.Stat(destPath)
	if err != nil && !os.IsNotExist(err) {
		return ctrl.Result{}, err
	}

	// If the destination path does not exist, download it now.
	if os.IsNotExist(err) {
		lastFetch := r.fetches[nn]
		lastBackoff := r.backoffs[nn]
		if time.Since(lastFetch) < lastBackoff {
			// If we're already in the middle of a backoff period, requeue.
			return ctrl.Result{RequeueAfter: lastBackoff}, nil
		}

		_, err := dlr.Download(importPath)
		if err != nil {
			backoff := r.nextBackoff(nn)
			backoffMsg := fmt.Sprintf("Downloading ExtensionRepo %s. Waiting %s before retrying. Error: %v", repo.Name, backoff, err)
			logger.Get(ctx).Errorf("%s", backoffMsg)
			_, updateErr := r.updateStatus(ctx, &repo, func(status *v1alpha1.ExtensionRepoStatus) {
				status.Error = backoffMsg
			})
			if updateErr != nil {
				return ctrl.Result{}, updateErr
			}

			return ctrl.Result{RequeueAfter: backoff}, nil
		}

		info, err = os.Stat(destPath)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update the status.
	delete(r.backoffs, nn)
	r.lastKnownDestPaths[nn] = destPath

	timeFetched := apis.NewTime(info.ModTime())
	return r.updateStatus(ctx, &repo, func(status *v1alpha1.ExtensionRepoStatus) {
		status.Error = ""
		status.LastFetchedAt = timeFetched
		status.Path = destPath
	})
}

// Step up the backoff after an error.
func (r *Reconciler) nextBackoff(nn types.NamespacedName) time.Duration {
	backoff := r.backoffs[nn]
	if backoff == 0 {
		backoff = 5 * time.Second
	} else {
		backoff = 2 * backoff
	}
	r.fetches[nn] = time.Now()
	r.backoffs[nn] = backoff
	return backoff
}

// Loosely inspired by controllerutil's Update status algorithm.
func (r *Reconciler) updateStatus(ctx context.Context, repo *v1alpha1.ExtensionRepo, mutateFn func(*v1alpha1.ExtensionRepoStatus)) (ctrl.Result, error) {
	update := repo.DeepCopy()
	mutateFn(&(update.Status))

	if apicmp.DeepEqual(update.Status, repo.Status) {
		return ctrl.Result{}, nil
	}

	err := r.ctrlClient.Status().Update(ctx, update)
	return ctrl.Result{}, err
}

func getImportPath(repo *v1alpha1.ExtensionRepo) (string, error) {
	// TODO(nick): Add file URLs
	url := repo.Spec.URL
	if strings.HasPrefix(url, "https://") {
		return strings.TrimPrefix(url, "https://"), nil
	}
	if strings.HasPrefix(url, "http://") {
		return strings.TrimPrefix(url, "http://"), nil
	}
	return "", fmt.Errorf("URL must start with 'https://': %v", url)
}
