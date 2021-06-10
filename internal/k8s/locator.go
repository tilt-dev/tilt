package k8s

import (
	"fmt"
	"reflect"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s/jsonpath"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type ImageLocator interface {
	// Checks whether two image locators are the same.
	EqualsImageLocator(other interface{}) bool

	// Find all the images in this entity.
	Extract(e K8sEntity) ([]reference.Named, error)

	// Matches the type of this entity.
	MatchesType(e K8sEntity) bool

	// Find all the images in this entity that match the given selector,
	// and replace them with the injected ref.
	// If the injected ref has a pull policy sibling, set it to the given pull policy.
	//
	// Returns a new entity with the injected ref.  Returns a boolean indicated
	// whether there was at least one successful injection.
	Inject(e K8sEntity, selector container.RefSelector, injectRef reference.Named, policy v1.PullPolicy) (K8sEntity, bool, error)

	ToSpec() v1alpha1.KubernetesImageLocator
}

const imagePullPolicyKey = "imagePullPolicy"

func LocatorMatchesOne(l ImageLocator, entities []K8sEntity) bool {
	for _, e := range entities {
		if l.MatchesType(e) {
			return true
		}
	}
	return false
}

type JSONPathImageLocator struct {
	selector ObjectSelector
	path     JSONPath
}

func MustJSONPathImageLocator(selector ObjectSelector, path string) *JSONPathImageLocator {
	locator, err := NewJSONPathImageLocator(selector, path)
	if err != nil {
		panic(err)
	}
	return locator
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

func (l *JSONPathImageLocator) ToSpec() v1alpha1.KubernetesImageLocator {
	return v1alpha1.KubernetesImageLocator{
		ObjectSelector: l.selector.ToSpec(),
		Path:           l.path.String(),
	}
}

func (l *JSONPathImageLocator) EqualsImageLocator(other interface{}) bool {
	otherL, ok := other.(*JSONPathImageLocator)
	if !ok {
		return false
	}

	return l.path.path == otherL.path.path &&
		l.selector.EqualsSelector(otherL.selector)
}

func (l *JSONPathImageLocator) MatchesType(e K8sEntity) bool {
	return l.selector.Matches(e)
}

func (l *JSONPathImageLocator) unpack(e K8sEntity) interface{} {
	if u, ok := e.Obj.(runtime.Unstructured); ok {
		return u.UnstructuredContent()
	}
	return e.Obj
}

func (l *JSONPathImageLocator) Extract(e K8sEntity) ([]reference.Named, error) {
	if !l.selector.Matches(e) {
		return nil, nil
	}

	// also look for images in any json paths that were specified for this entity
	images, err := l.path.FindStrings(l.unpack(e))
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

func (l *JSONPathImageLocator) Inject(e K8sEntity, selector container.RefSelector, injectRef reference.Named, pullPolicy v1.PullPolicy) (K8sEntity, bool, error) {
	if !l.selector.Matches(e) {
		return e, false, nil
	}

	modified := false
	err := l.path.VisitStrings(l.unpack(e), func(val jsonpath.Value, str string) error {
		ref, err := container.ParseNamed(str)
		if err != nil {
			return nil
		}

		if selector.Matches(ref) {
			val.Set(reflect.ValueOf(container.FamiliarString(injectRef)))
			modified = true

			if pullPolicyVal, ok := val.Sibling(imagePullPolicyKey); ok && pullPolicyVal.CanSet() {
				pullPolicyVal.Set(reflect.ValueOf(string(pullPolicy)))
			}
		}
		return nil
	})
	if err != nil {
		return e, false, err
	}
	return e, modified, nil
}

var _ ImageLocator = &JSONPathImageLocator{}

type JSONPathImageObjectLocator struct {
	selector  ObjectSelector
	path      JSONPath
	repoField string
	tagField  string
}

func MustJSONPathImageObjectLocator(selector ObjectSelector, path, repoField, tagField string) *JSONPathImageObjectLocator {
	locator, err := NewJSONPathImageObjectLocator(selector, path, repoField, tagField)
	if err != nil {
		panic(err)
	}
	return locator
}

func NewJSONPathImageObjectLocator(selector ObjectSelector, path, repoField, tagField string) (*JSONPathImageObjectLocator, error) {
	p, err := NewJSONPath(path)
	if err != nil {
		return nil, err
	}
	return &JSONPathImageObjectLocator{
		selector:  selector,
		path:      p,
		repoField: repoField,
		tagField:  tagField,
	}, nil
}

func (l *JSONPathImageObjectLocator) EqualsImageLocator(other interface{}) bool {
	otherL, ok := other.(*JSONPathImageObjectLocator)
	if !ok {
		return false
	}
	return l.path.path == otherL.path.path &&
		l.repoField == otherL.repoField &&
		l.tagField == otherL.tagField &&
		l.selector.EqualsSelector(otherL.selector)
}

func (l *JSONPathImageObjectLocator) MatchesType(e K8sEntity) bool {
	return l.selector.Matches(e)
}

func (l *JSONPathImageObjectLocator) unpack(e K8sEntity) interface{} {
	if u, ok := e.Obj.(runtime.Unstructured); ok {
		return u.UnstructuredContent()
	}
	return e.Obj
}

func (l *JSONPathImageObjectLocator) extractImageFromMap(val jsonpath.Value) (reference.Named, error) {
	m, ok := val.Interface().(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("May only match maps (json_path=%q)\nGot Type: %s\nGot Value: %s",
			l.path.path, val.Type(), val)
	}

	repoField, ok := m[l.repoField].(string)
	imageString := ""
	if ok {
		imageString = repoField
	}

	tagField, ok := m[l.tagField].(string)
	if ok && tagField != "" {
		imageString = fmt.Sprintf("%s:%s", repoField, tagField)
	}

	return container.ParseNamed(imageString)
}

func (l *JSONPathImageObjectLocator) Extract(e K8sEntity) ([]reference.Named, error) {
	if !l.selector.Matches(e) {
		return nil, nil
	}

	result := make([]reference.Named, 0)
	err := l.path.Visit(l.unpack(e), func(val jsonpath.Value) error {
		ref, err := l.extractImageFromMap(val)
		if err != nil {
			return err
		}
		result = append(result, ref)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// pullPolicy is ignored for this injector for now, since it's less standard
// if it turns out there's a demand for this, we can plumb it through to image_object
func (l *JSONPathImageObjectLocator) Inject(e K8sEntity, selector container.RefSelector, injectRef reference.Named, _ v1.PullPolicy) (K8sEntity, bool, error) {
	if !l.selector.Matches(e) {
		return e, false, nil
	}

	tagged, isTagged := injectRef.(reference.Tagged)

	modified := false
	err := l.path.Visit(l.unpack(e), func(val jsonpath.Value) error {
		ref, err := l.extractImageFromMap(val)
		if err != nil {
			return err
		}
		if selector.Matches(ref) {
			m := val.Interface().(map[string]interface{})
			m[l.repoField] = reference.FamiliarName(injectRef)
			if isTagged {
				m[l.tagField] = tagged.Tag()
			}
			modified = true
		}
		return nil
	})
	if err != nil {
		return e, false, err
	}
	return e, modified, nil
}

func (l *JSONPathImageObjectLocator) ToSpec() v1alpha1.KubernetesImageLocator {
	return v1alpha1.KubernetesImageLocator{
		ObjectSelector: l.selector.ToSpec(),
		Path:           l.path.String(),
		Object: &v1alpha1.KubernetesImageObjectDescriptor{
			RepoField: l.repoField,
			TagField:  l.tagField,
		},
	}
}

var _ ImageLocator = &JSONPathImageObjectLocator{}
