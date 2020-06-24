package k8s

import (
	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/tilt-dev/tilt/internal/container"
	"k8s.io/apimachinery/pkg/runtime"
)

type ImageLocator interface {
	// Find all the images in this entity.
	Extract(e K8sEntity) ([]reference.Named, error)

	// Matches the type of this entity.
	MatchesType(e K8sEntity) bool

	// Find all the images in this entity that match the given selector,
	// and replace them with the injected ref.
	//
	// Returns a new entity with the injected ref.  Returns a boolean indicated
	// whether there was at least one successful injection.
	//Inject(e K8sEntity, selector container.RefSelector, injectRef reference.Named) (K8sEntity, bool, error)
}

type JSONPathImageLocator struct {
	selector ObjectSelector
	path     JSONPath
}

func NewJSONPathImageLocator(selector ObjectSelector, path string) (*JSONPathImageLocator, error) {
	p, err := NewJSONPath(path)
	if err != nil {
		return nil, err
	}
	return &JSONPathImageLocator{
		selector: selector,
		path:     p,
	}, nil
}

func (l *JSONPathImageLocator) MatchesType(e K8sEntity) bool {
	return l.selector.Matches(e)
}

func (l *JSONPathImageLocator) Extract(e K8sEntity) ([]reference.Named, error) {
	if !l.selector.Matches(e) {
		return nil, nil
	}

	var obj interface{}
	if u, ok := e.Obj.(runtime.Unstructured); ok {
		obj = u.UnstructuredContent()
	} else {
		obj = e.Obj
	}

	// also look for images in any json paths that were specified for this entity
	images, err := l.path.FindStrings(obj)
	if err != nil {
		return nil, err
	}

	result := make([]reference.Named, 0, len(images))
	for _, image := range images {
		ref, err := container.ParseNamed(image)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing image '%s' at json path '%s'", image, l.path)
		}
		result = append(result, ref)
	}
	return result, nil
}

var _ ImageLocator = &JSONPathImageLocator{}
