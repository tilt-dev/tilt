package extensionrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

type Downloader interface {
	DestinationPath(pkg string) string
	Download(pkg string) (string, error)
	HeadRef(pgs string) (string, error)
}

type Reconciler struct {
	ctrlClient ctrlclient.Client
	dlr        Downloader

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
		dlr:                get.NewDownloader(dlrPath),
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

	if strings.HasPrefix(repo.Spec.URL, "file://") {
		return r.reconcileFileRepo(ctx, &repo, strings.TrimPrefix(repo.Spec.URL, "file://"))
	}

	// Check that the URL is valid.
	importPath, err := getDownloaderImportPath(&repo)
	if err != nil {
		return r.updateError(ctx, &repo, fmt.Sprintf("invalid: %v", err))
	}

	return r.reconcileDownloaderRepo(ctx, nn, &repo, importPath)
}

// Reconcile a repo that lives on disk, and shouldn't otherwise be modified.
func (r *Reconciler) reconcileFileRepo(ctx context.Context, repo *v1alpha1.ExtensionRepo, filePath string) (reconcile.Result, error) {
	if !filepath.IsAbs(filePath) {
		msg := fmt.Sprintf("file paths must be absolute. Url: %s", repo.Spec.URL)
		return r.updateError(ctx, repo, msg)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		msg := fmt.Sprintf("loading: %v", err)
		return r.updateError(ctx, repo, msg)
	}

	if !info.IsDir() {
		msg := "loading: not a directory"
		return r.updateError(ctx, repo, msg)
	}

	timeFetched := apis.NewTime(info.ModTime())
	return r.updateStatus(ctx, repo, func(status *v1alpha1.ExtensionRepoStatus) {
		status.Error = ""
		status.LastFetchedAt = timeFetched
		status.Path = filePath
	})
}

// Reconcile a repo that we need to fetch remotely, and store
// under ~/.tilt-dev.
func (r *Reconciler) reconcileDownloaderRepo(ctx context.Context, nn types.NamespacedName,
	repo *v1alpha1.ExtensionRepo, importPath string) (reconcile.Result, error) {
	getDlr, ok := r.dlr.(*get.Downloader)
	if ok {
		getDlr.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)
	}

	destPath := r.dlr.DestinationPath(importPath)
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

		_, err := r.dlr.Download(importPath)
		if err != nil {
			backoff := r.nextBackoff(nn)
			backoffMsg := fmt.Sprintf("download error: waiting %s before retrying. Original error: %v", backoff, err)
			_, updateErr := r.updateError(ctx, repo, backoffMsg)
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
	return r.updateStatus(ctx, repo, func(status *v1alpha1.ExtensionRepoStatus) {
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

// Update status with an error message, logging the error.
func (r *Reconciler) updateError(ctx context.Context, repo *v1alpha1.ExtensionRepo, errorMsg string) (ctrl.Result, error) {
	update := repo.DeepCopy()
	update.Status.Error = errorMsg
	update.Status.Path = ""

	if apicmp.DeepEqual(update.Status, repo.Status) {
		return ctrl.Result{}, nil
	}

	logger.Get(ctx).Errorf("extensionrepo %s: %s", repo.Name, errorMsg)

	err := r.ctrlClient.Status().Update(ctx, update)
	return ctrl.Result{}, err
}

func getDownloaderImportPath(repo *v1alpha1.ExtensionRepo) (string, error) {
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
