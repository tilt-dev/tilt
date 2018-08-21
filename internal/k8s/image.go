package k8s

import (
	"fmt"
	"reflect"

	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	"k8s.io/api/core/v1"
)

// Returns: the new entity, whether anything was replaced, and an error.
func InjectImageDigestWithStrings(entity K8sEntity, original string, newDigest string, policy v1.PullPolicy) (K8sEntity, bool, error) {
	originalRef, err := reference.ParseNamed(original)
	if err != nil {
		return K8sEntity{}, false, err
	}

	d, err := digest.Parse(newDigest)
	if err != nil {
		return K8sEntity{}, false, err
	}

	canonicalRef, err := reference.WithDigest(originalRef, d)
	if err != nil {
		return K8sEntity{}, false, err
	}

	return InjectImageDigest(entity, canonicalRef, policy)
}

// Iterate through the fields of a k8s entity and
// replace the image pull policy on all images.
func InjectImagePullPolicy(entity K8sEntity, policy v1.PullPolicy) (K8sEntity, error) {
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
func InjectImageDigest(entity K8sEntity, canonicalRef reference.Canonical, policy v1.PullPolicy) (K8sEntity, bool, error) {

	containers, err := extractContainers(&entity)
	if err != nil {
		return K8sEntity{}, false, err
	}

	replaced := false
	for _, container := range containers {
		existingRef, err := reference.ParseNamed(container.Image)
		if err != nil {
			return K8sEntity{}, false, err
		}

		if existingRef.Name() == canonicalRef.Name() {
			container.Image = canonicalRef.String()
			container.ImagePullPolicy = policy
			replaced = true
		}
	}
	return entity, replaced, nil
}

// Get pointers to all the container specs in this object.
func extractContainers(obj interface{}) ([]*v1.Container, error) {
	cType := reflect.TypeOf(v1.Container{})
	v := reflect.ValueOf(obj)
	result := make([]*v1.Container, 0)

	// Recursively iterate over the struct fields.
	var extract func(v reflect.Value) error
	extract = func(v reflect.Value) error {
		switch v.Kind() {
		case reflect.Ptr, reflect.Interface:
			if v.IsNil() {
				return nil
			}
			return extract(v.Elem())

		case reflect.Struct:
			if v.Type() == cType {
				if !v.CanAddr() {
					return fmt.Errorf("Error addressing: %v", v)
				}
				ptr, ok := v.Addr().Interface().(*v1.Container)
				if !ok {
					return fmt.Errorf("Error addressing: %v", v)
				}
				result = append(result, ptr)
				return nil
			}

			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				err := extract(field)
				if err != nil {
					return err
				}
			}
			return nil

		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				field := v.Index(i)
				err := extract(field)
				if err != nil {
					return err
				}
			}
			return nil

		}
		return nil
	}

	err := extract(v)
	if err != nil {
		return nil, err
	}
	return result, nil
}
