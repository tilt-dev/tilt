package k8s

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/windmilleng/tilt/internal/container"
)

// Iterate through the fields of a k8s entity and
// replace the image pull policy on all images.
func InjectImagePullPolicy(entity K8sEntity, policy v1.PullPolicy) (K8sEntity, error) {
	entity = entity.DeepCopy()
	containers, err := extractContainers(&entity)
	if err != nil {
		return K8sEntity{}, err
	}

	for _, container := range containers {
		container.ImagePullPolicy = policy
	}
	return entity, nil
}

// Iterate through the fields of a k8s entity and
// replace a image name with its digest.
//
// policy: The pull policy to set on the replaced image.
//   When working with a local k8s cluster, we want to set this to Never,
//   to ensure that k8s fails hard if the image is missing from docker.
//
// Returns: the new entity, whether the image was replaced, and an error.
func InjectImageDigest(entity K8sEntity, selector container.RefSelector, injectRef reference.Named, policy v1.PullPolicy) (K8sEntity, bool, error) {
	entity = entity.DeepCopy()

	// NOTE(nick): For some reason, if you have a reference with a digest,
	// kubernetes will never find it in the local registry and always tries to do a
	// pull. It's not clear to me why it behaves this way.
	//
	// There is not a simple way to resolve this problem at this level of the
	// API. In some cases, the digest won't matter and the name/tag will be
	// enough. In other cases, the digest will be critical if we don't have good
	// synchronization that the name/tag currently matches the digest.
	//
	// For now, we try to detect this case and push the error up to the caller.
	_, hasDigest := injectRef.(reference.Digested)
	if hasDigest && policy == v1.PullNever {
		return K8sEntity{}, false, fmt.Errorf("INTERNAL TILT ERROR: Cannot set PullNever with digest")
	}

	replaced := false

	entity, r, err := injectImageDigestInContainers(entity, selector, injectRef, policy)
	if err != nil {
		return K8sEntity{}, false, err
	}
	if r {
		replaced = true
	}

	entity, r, err = injectImageDigestInEnvVars(entity, selector, injectRef)
	if err != nil {
		return K8sEntity{}, false, err
	}
	if r {
		replaced = true
	}

	entity, r, err = injectImageDigestInUnstructured(entity, injectRef)
	if err != nil {
		return K8sEntity{}, false, err
	}
	if r {
		replaced = true
	}

	return entity, replaced, nil
}

func injectImageDigestInContainers(entity K8sEntity, selector container.RefSelector, injectRef reference.Named, policy v1.PullPolicy) (K8sEntity, bool, error) {
	containers, err := extractContainers(&entity)
	if err != nil {
		return K8sEntity{}, false, err
	}

	replaced := false
	for _, c := range containers {
		existingRef, err := container.ParseNamed(c.Image)
		if err != nil {
			return K8sEntity{}, false, err
		}

		if selector.Matches(existingRef) {
			c.Image = injectRef.String()
			c.ImagePullPolicy = policy
			replaced = true
		}
	}

	return entity, replaced, nil
}

func injectImageDigestInEnvVars(entity K8sEntity, selector container.RefSelector, injectRef reference.Named) (K8sEntity, bool, error) {
	envVars, err := extractEnvVars(&entity)
	if err != nil {
		return K8sEntity{}, false, err
	}

	replaced := false
	for _, envVar := range envVars {
		existingRef, err := container.ParseNamed(envVar.Value)
		if err != nil || existingRef == nil {
			continue
		}

		if selector.Matches(existingRef) {
			envVar.Value = injectRef.String()
			replaced = true
		}
	}

	return entity, replaced, nil
}

func injectImageInUnstructuredInterface(ui interface{}, injectRef reference.Named) (interface{}, bool) {
	switch x := ui.(type) {
	case map[string]interface{}:
		replaced := false
		for k, v := range x {
			newV, r := injectImageInUnstructuredInterface(v, injectRef)
			x[k] = newV
			if r {
				replaced = true
			}
		}
		return x, replaced
	case []interface{}:
		replaced := false
		for i, v := range x {
			newV, r := injectImageInUnstructuredInterface(v, injectRef)
			x[i] = newV
			if r {
				replaced = true
			}
		}
		return x, replaced
	case string:
		ref, err := container.ParseNamed(x)
		if err == nil && ref.Name() == injectRef.Name() {
			return injectRef.String(), true
		} else {
			return x, false
		}
	default:
		return ui, false
	}
}

func injectImageDigestInUnstructured(entity K8sEntity, injectRef reference.Named) (K8sEntity, bool, error) {
	u, ok := entity.Obj.(runtime.Unstructured)
	if !ok {
		return entity, false, nil
	}

	n, replaced := injectImageInUnstructuredInterface(u.UnstructuredContent(), injectRef)

	u.SetUnstructuredContent(n.(map[string]interface{}))

	entity.Obj = u
	return entity, replaced, nil
}

// HasImage indicates whether the given entity is tagged with the given image.
func (e K8sEntity) HasImage(image container.RefSelector, imageJSONPaths []JSONPath) (bool, error) {
	images, err := e.FindImages(imageJSONPaths)
	if err != nil {
		fmt.Printf("error in FindImages: %+v\n", err)
		return false, err
	}

	for _, existingRef := range images {
		if image.Matches(existingRef) {
			return true, nil
		}
	}

	return false, nil
}

func (e K8sEntity) FindImages(imageJSONPaths []JSONPath) ([]reference.Named, error) {
	var result []reference.Named

	// Look for images in instances of Container
	containers, err := extractContainers(&e)
	if err != nil {
		return nil, err
	}
	for _, c := range containers {
		ref, err := container.ParseNamed(c.Image)
		if err != nil {
			return nil, err
		}

		result = append(result, ref)
	}

	var obj interface{}
	if u, ok := e.Obj.(runtime.Unstructured); ok {
		obj = u.UnstructuredContent()
	} else {
		obj = e.Obj
	}

	// also look for images in any json paths that were specified for this entity
	for _, path := range imageJSONPaths {
		image, err := path.Execute(obj)
		if err != nil {
			return nil, errors.Wrapf(err, "error applying json path '%s'", path)
		}
		ref, err := container.ParseNamed(image)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing image '%s' at json path '%s'", image, path)
		}
		result = append(result, ref)
	}

	return result, nil
}

func PodContainsRef(pod v1.PodSpec, selector container.RefSelector) (bool, error) {
	cRef, err := FindImageRefMatching(pod, selector)
	if err != nil {
		return false, err
	}

	return cRef != nil, nil
}

func FindImageRefMatching(pod v1.PodSpec, selector container.RefSelector) (reference.Named, error) {
	for _, c := range pod.Containers {
		cRef, err := container.ParseNamed(c.Image)
		if err != nil {
			return nil, errors.Wrap(err, "FindImageRefMatching")
		}

		if selector.Matches(cRef) {
			return cRef, nil
		}
	}
	return nil, nil
}

func FindImageNamedTaggedMatching(pod v1.PodSpec, selector container.RefSelector) (reference.NamedTagged, error) {
	cRef, err := FindImageRefMatching(pod, selector)
	if err != nil {
		return nil, err
	}

	cTagged, ok := cRef.(reference.NamedTagged)
	if !ok {
		return nil, nil
	}

	return cTagged, nil
}
