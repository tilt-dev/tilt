package k8s

import (
	"crypto"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

const TiltPodTemplateHashLabel = "tilt-pod-template-hash"

type PodTemplateSpecHash string

func HashPodTemplateSpec(spec *v1.PodTemplateSpec) (PodTemplateSpecHash, error) {
	data, err := defaultJSONIterator.Marshal(spec)
	if err != nil {
		return "", errors.Wrap(err, "serializing spec to json")
	}

	h := crypto.SHA1.New()
	h.Write(data)
	return PodTemplateSpecHash(fmt.Sprintf("%x", h.Sum(nil)[:10])), nil
}

// Iterate through the fields of a k8s entity and add the pod template spec hash on all
// pod template specs
func InjectPodTemplateSpecHash(entity K8sEntity) (K8sEntity, []PodTemplateSpecHash, error) {
	entity = entity.DeepCopy()
	templateSpecs, err := ExtractPodTemplateSpec(&entity)
	if err != nil {
		return K8sEntity{}, nil, err
	}

	var hashes []PodTemplateSpecHash

	for _, ts := range templateSpecs {
		h, err := HashPodTemplateSpec(ts)
		if err != nil {
			return K8sEntity{}, nil, errors.Wrap(err, "calculating hash")
		}
		ts.Labels[TiltPodTemplateHashLabel] = string(h)
		hashes = append(hashes, h)
	}

	return entity, hashes, nil
}
