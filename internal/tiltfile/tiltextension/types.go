package tiltextension

import (
	"context"
	"path/filepath"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Interfaces with just the reconciler methods we need.
type ExtRepoReconciler interface {
	ForceApply(ctx context.Context, repo *v1alpha1.ExtensionRepo) v1alpha1.ExtensionRepoStatus
}

type ExtReconciler interface {
	ForceApply(ext *v1alpha1.Extension, repo *v1alpha1.ExtensionRepo) v1alpha1.ExtensionStatus
}

// Fake versions of these interfaces.
type FakeExtRepoReconciler struct {
	path  string
	Error string
}

func NewFakeExtRepoReconciler(path string) *FakeExtRepoReconciler {
	return &FakeExtRepoReconciler{path: path}
}

func (r *FakeExtRepoReconciler) ForceApply(ctx context.Context, repo *v1alpha1.ExtensionRepo) v1alpha1.ExtensionRepoStatus {
	if r.Error != "" {
		return v1alpha1.ExtensionRepoStatus{Error: r.Error}
	}
	return v1alpha1.ExtensionRepoStatus{
		Path: filepath.Join(r.path, filepath.Base(repo.Spec.URL)),
	}
}

type FakeExtReconciler struct {
	path  string
	Error string
}

func NewFakeExtReconciler(path string) *FakeExtReconciler {
	return &FakeExtReconciler{path: path}
}

func (r *FakeExtReconciler) ForceApply(ext *v1alpha1.Extension, repo *v1alpha1.ExtensionRepo) v1alpha1.ExtensionStatus {
	if r.Error != "" {
		return v1alpha1.ExtensionStatus{Error: r.Error}
	}
	return v1alpha1.ExtensionStatus{
		Path: filepath.Join(repo.Status.Path, repo.Spec.Path, ext.Spec.RepoPath, "Tiltfile"),
	}
}
