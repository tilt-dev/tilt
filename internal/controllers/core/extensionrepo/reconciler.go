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
	HeadRef(pkg string) (string, error)
	RefSync(pkg string, ref string) error
}

type Reconciler struct {
	ctrlClient ctrlclient.Client
	dlr        Downloader

	repoStates map[types.NamespacedName]*repoState
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
		ctrlClient: ctrlClient,
		dlr:        get.NewDownloader(dlrPath),
		repoStates: make(map[types.NamespacedName]*repoState),
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

	isDelete := apierrors.IsNotFound(err) || !repo.ObjectMeta.DeletionTimestamp.IsZero()
	needsCleanup := isDelete

	// If the spec has changed, clear the current repo state.
	state, ok := r.repoStates[nn]
	if ok && !apicmp.DeepEqual(state.spec, repo.Spec) {
		needsCleanup = true
	}

	if needsCleanup {
		// If a repo is deleted, delete it on disk.
		//
		// This is simple, but not accurate.
		// 1) It will garbage collect too aggressively if two extension repo
		//    objects point to the same URL.
		// 2) If we "miss" a delete event, the repo will never get cleaned up.
		//
		// A "real" implementation would use the on-disk repos as the source of
		// truth, and garbage collect ones with no remaining refs.
		if state != nil && state.lastSuccessfulDestPath != "" {
			err := os.RemoveAll(state.lastSuccessfulDestPath)
			if err != nil && !os.IsNotExist(err) {
				return ctrl.Result{}, err
			}
		}
		delete(r.repoStates, nn)
	}

	if isDelete {
		return ctrl.Result{}, nil
	}

	state, ok = r.repoStates[nn]
	if !ok {
		state = &repoState{spec: repo.Spec}
		r.repoStates[nn] = state
	}

	if strings.HasPrefix(repo.Spec.URL, "file://") {
		return r.reconcileFileRepo(ctx, &repo, strings.TrimPrefix(repo.Spec.URL, "file://"))
	}

	// Check that the URL is valid.
	importPath, err := getDownloaderImportPath(&repo)
	if err != nil {
		return r.updateError(ctx, &repo, fmt.Sprintf("invalid: %v", err))
	}

	return r.reconcileDownloaderRepo(ctx, nn, &repo, importPath, state)
}

// Reconcile a repo that lives on disk, and shouldn't otherwise be modified.
func (r *Reconciler) reconcileFileRepo(ctx context.Context, repo *v1alpha1.ExtensionRepo, filePath string) (reconcile.Result, error) {
	if repo.Spec.Ref != "" {
		return r.updateError(ctx, repo, "spec.ref not supported on file:// repos")
	}

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
	repo *v1alpha1.ExtensionRepo, importPath string, state *repoState) (reconcile.Result, error) {
	getDlr, ok := r.dlr.(*get.Downloader)
	if ok {
		getDlr.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)
	}

	destPath := r.dlr.DestinationPath(importPath)
	_, err := os.Stat(destPath)
	if err != nil && !os.IsNotExist(err) {
		return ctrl.Result{}, err
	}

	// If the directory exists and has already been fetched successfully during this session,
	// no reconciliation is needed.
	exists := err == nil
	if exists && state.lastSuccessfulDestPath != "" {
		return ctrl.Result{}, nil
	}

	lastFetch := state.lastFetch
	lastBackoff := state.backoff
	if time.Since(lastFetch) < lastBackoff {
		// If we're already in the middle of a backoff period, requeue.
		return ctrl.Result{RequeueAfter: lastBackoff}, nil
	}

	state.lastFetch = time.Now()

	_, err = r.dlr.Download(importPath)
	if err != nil {
		backoff := state.nextBackoff()
		backoffMsg := fmt.Sprintf("download error: waiting %s before retrying. Original error: %v", backoff, err)
		_, updateErr := r.updateError(ctx, repo, backoffMsg)
		if updateErr != nil {
			return ctrl.Result{}, updateErr
		}

		return ctrl.Result{RequeueAfter: backoff}, nil
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return ctrl.Result{}, err
	}

	if repo.Spec.Ref != "" {
		err := r.dlr.RefSync(importPath, repo.Spec.Ref)
		if err != nil {
			return r.updateError(ctx, repo, fmt.Sprintf("sync to ref %s: %v", repo.Spec.Ref, err))
		}
	}

	ref, err := r.dlr.HeadRef(importPath)
	if err != nil {
		return r.updateError(ctx, repo, fmt.Sprintf("determining head: %v", err))
	}

	// Update the status.
	state.backoff = 0
	state.lastSuccessfulDestPath = destPath

	timeFetched := apis.NewTime(info.ModTime())
	return r.updateStatus(ctx, repo, func(status *v1alpha1.ExtensionRepoStatus) {
		status.Error = ""
		status.LastFetchedAt = timeFetched
		status.Path = destPath
		status.CheckoutRef = ref
	})
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

type repoState struct {
	spec                   v1alpha1.ExtensionRepoSpec
	lastFetch              time.Time
	backoff                time.Duration
	lastSuccessfulDestPath string
}

// Step up the backoff after an error.
func (s *repoState) nextBackoff() time.Duration {
	backoff := s.backoff
	if backoff == 0 {
		backoff = 5 * time.Second
	} else {
		backoff = 2 * backoff
	}
	s.backoff = backoff
	return backoff
}
