package extensionrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tilt-dev/go-get"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/xdg"
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
	st         store.RStore
	dlr        Downloader
	mu         sync.Mutex

	repoStates map[types.NamespacedName]*repoState
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ExtensionRepo{})

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, st store.RStore, base xdg.Base) (*Reconciler, error) {
	dlrPath, err := base.DataFile(tiltModulesRelDir)
	if err != nil {
		return nil, fmt.Errorf("creating extensionrepo controller: %v", err)
	}
	return &Reconciler{
		ctrlClient: ctrlClient,
		st:         st,
		dlr:        get.NewDownloader(dlrPath),
		repoStates: make(map[types.NamespacedName]*repoState),
	}, nil
}

// Downloads extension repos.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	nn := request.NamespacedName

	var repo v1alpha1.ExtensionRepo
	err := r.ctrlClient.Get(ctx, nn, &repo)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	ctx = store.MustObjectLogHandler(ctx, r.st, &repo)

	result, state, err := r.apply(ctx, nn, &repo)
	if err != nil {
		return ctrl.Result{}, err
	}
	if state == nil {
		return ctrl.Result{}, nil
	}

	err = r.maybeUpdateStatus(ctx, &repo, state)
	if err != nil {
		return ctrl.Result{}, err
	}
	return result, nil
}

// Reconciles the extension repo without reading or writing from the API server.
// Returns the resolved status.
// Exposed for outside callers.
func (r *Reconciler) ForceApply(ctx context.Context, repo *v1alpha1.ExtensionRepo) v1alpha1.ExtensionRepoStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	nn := types.NamespacedName{Name: repo.Name, Namespace: repo.Namespace}
	_, state, err := r.apply(ctx, nn, repo)
	if err != nil {
		return v1alpha1.ExtensionRepoStatus{Error: err.Error()}
	}
	if state == nil {
		return v1alpha1.ExtensionRepoStatus{Error: "internal error: could not reconcile"}
	}
	return state.status
}

// Reconciles the extension repo without reading or writing from the API server.
// Caller must hold the mutex.
// Returns a nil state if the repo is being deleted.
// Returns an error if the reconcile should be retried.
func (r *Reconciler) apply(ctx context.Context, nn types.NamespacedName, repo *v1alpha1.ExtensionRepo) (ctrl.Result, *repoState, error) {
	isDelete := repo.Name == "" || !repo.DeletionTimestamp.IsZero()
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
				return ctrl.Result{}, nil, err
			}
		}
		delete(r.repoStates, nn)
	}

	if isDelete {
		return ctrl.Result{}, nil, nil
	}

	state, ok = r.repoStates[nn]
	if !ok {
		state = &repoState{spec: repo.Spec}
		r.repoStates[nn] = state
	}

	// Keep track of the Result in case it contains Requeue instructions.
	var result ctrl.Result
	if strings.HasPrefix(repo.Spec.URL, "file://") {
		r.reconcileFileRepo(ctx, state, strings.TrimPrefix(repo.Spec.URL, "file://"))
	} else {
		// Check that the URL is valid.
		importPath, err := getDownloaderImportPath(repo)
		if err != nil {
			state.status = v1alpha1.ExtensionRepoStatus{Error: fmt.Sprintf("invalid: %v", err)}
		} else {
			result = r.reconcileDownloaderRepo(ctx, state, importPath)
		}
	}
	return result, state, nil
}

// Reconcile a repo that lives on disk, and shouldn't otherwise be modified.
func (r *Reconciler) reconcileFileRepo(ctx context.Context, state *repoState, filePath string) {
	if state.spec.Ref != "" {
		state.status = v1alpha1.ExtensionRepoStatus{Error: "spec.ref not supported on file:// repos"}
		return
	}

	if !filepath.IsAbs(filePath) {
		state.status = v1alpha1.ExtensionRepoStatus{
			Error: fmt.Sprintf("file paths must be absolute. Url: %s", state.spec.URL),
		}
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		state.status = v1alpha1.ExtensionRepoStatus{Error: fmt.Sprintf("loading: %v", err)}
		return
	}

	if !info.IsDir() {
		state.status = v1alpha1.ExtensionRepoStatus{Error: "loading: not a directory"}
		return
	}

	timeFetched := apis.NewTime(info.ModTime())
	state.status = v1alpha1.ExtensionRepoStatus{LastFetchedAt: timeFetched, Path: filePath}
}

// Reconcile a repo that we need to fetch remotely, and store
// under ~/.tilt-dev.
func (r *Reconciler) reconcileDownloaderRepo(ctx context.Context, state *repoState, importPath string) reconcile.Result {
	getDlr, ok := r.dlr.(*get.Downloader)
	if ok {
		getDlr.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)
	}

	destPath := r.dlr.DestinationPath(importPath)
	_, err := os.Stat(destPath)
	if err != nil && !os.IsNotExist(err) {
		state.status = v1alpha1.ExtensionRepoStatus{Error: fmt.Sprintf("loading download destination: %v", err)}
		return ctrl.Result{}
	}

	// If the directory exists and has already been fetched successfully during this session,
	// no reconciliation is needed.
	exists := err == nil
	if exists && state.lastSuccessfulDestPath != "" {
		return ctrl.Result{}
	}

	lastFetch := state.lastFetch
	lastBackoff := state.backoff
	if time.Since(lastFetch) < lastBackoff {
		// If we're already in the middle of a backoff period, requeue.
		return ctrl.Result{RequeueAfter: lastBackoff}
	}

	state.lastFetch = time.Now()

	_, err = r.dlr.Download(importPath)
	if err != nil {
		backoff := state.nextBackoff()
		backoffMsg := fmt.Sprintf("download error: waiting %s before retrying. Original error: %v", backoff, err)
		state.status = v1alpha1.ExtensionRepoStatus{Error: backoffMsg}
		return ctrl.Result{RequeueAfter: backoff}
	}

	info, err := os.Stat(destPath)
	if err != nil {
		state.status = v1alpha1.ExtensionRepoStatus{Error: fmt.Sprintf("verifying download destination: %v", err)}
		return ctrl.Result{}
	}

	if state.spec.Ref != "" {
		err := r.dlr.RefSync(importPath, state.spec.Ref)
		if err != nil {
			state.status = v1alpha1.ExtensionRepoStatus{Error: fmt.Sprintf("sync to ref %s: %v", state.spec.Ref, err)}
			return ctrl.Result{}
		}
	}

	ref, err := r.dlr.HeadRef(importPath)
	if err != nil {
		state.status = v1alpha1.ExtensionRepoStatus{Error: fmt.Sprintf("determining head: %v", err)}
		return ctrl.Result{}
	}

	// Update the status.
	state.backoff = 0
	state.lastSuccessfulDestPath = destPath

	timeFetched := apis.NewTime(info.ModTime())
	state.status = v1alpha1.ExtensionRepoStatus{
		LastFetchedAt: timeFetched,
		Path:          destPath,
		CheckoutRef:   ref,
	}
	return ctrl.Result{}
}

// Loosely inspired by controllerutil's Update status algorithm.
func (r *Reconciler) maybeUpdateStatus(ctx context.Context, repo *v1alpha1.ExtensionRepo, state *repoState) error {
	if apicmp.DeepEqual(repo.Status, state.status) {
		return nil
	}

	oldError := repo.Status.Error
	newError := state.status.Error
	update := repo.DeepCopy()
	update.Status = *(state.status.DeepCopy())

	err := r.ctrlClient.Status().Update(ctx, update)
	if err != nil {
		return err
	}

	if newError != "" && oldError != newError {
		logger.Get(ctx).Errorf("extensionrepo %s: %s", repo.Name, newError)
	}
	return err
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

	status v1alpha1.ExtensionRepoStatus
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
