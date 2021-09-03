package v1alpha1

import (
	"go.starlark.net/starlark"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// TODO(nick): Autogenerate this. Avoid writing any custom code or handling here.
func (p Plugin) extensionRepo(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var labels value.StringStringMap
	var annotations value.StringStringMap
	var url string
	var ref string
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs,
		"name", &name,
		"labels?", &labels,
		"annotations?", &annotations,
		"url?", &url,
		"ref?", &ref,
	)
	if err != nil {
		return nil, err
	}

	obj := &v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: url,
			Ref: ref,
		},
	}

	return p.register(t, obj)
}

func (p Plugin) extension(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var labels value.StringStringMap
	var annotations value.StringStringMap
	var repoName string
	var repoPath string
	var specArgs value.StringList
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs,
		"name", &name,
		"labels?", &labels,
		"annotations?", &annotations,
		"repo_name?", &repoName,
		"repo_path?", &repoPath,
		"args?", &specArgs,
	)
	if err != nil {
		return nil, err
	}

	obj := &v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: repoName,
			RepoPath: repoPath,
			Args:     specArgs,
		},
	}

	return p.register(t, obj)
}
