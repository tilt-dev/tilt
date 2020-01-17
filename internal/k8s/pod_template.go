package k8s

import (
	"crypto"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

const TiltPodTemplateHashLabel = "tilt.dev/pod-template-hash"

type PodTemplateSpecHash string

func HashPodTemplateSpec(spec *v1.PodTemplateSpec) (PodTemplateSpecHash, error) {
	data, err := defaultJSONIterator.Marshal(spec)
	if err != nil {
		return "", errors.Wrap(err, "serializing spec to json")
	}

	h := crypto.SHA1.New()
	_, err = h.Write(data)
	if err != nil {
		return "", errors.Wrap(err, "writing to hash")
	}
	return PodTemplateSpecHash(fmt.Sprintf("%x", h.Sum(nil)[:10])), nil
}

// Iterate through the fields of a k8s entity and add the pod template spec hash on all
// pod template specs
func InjectPodTemplateSpecHashes(entity K8sEntity) (K8sEntity, error) {
	entity = entity.DeepCopy()
	templateSpecs, err := ExtractPodTemplateSpec(&entity)
	if err != nil {
		return K8sEntity{}, err
	}

	for _, ts := range templateSpecs {
		h, err := HashPodTemplateSpec(ts)
		if err != nil {
			return K8sEntity{}, errors.Wrap(err, "calculating hash")
		}
		ts.Labels[TiltPodTemplateHashLabel] = string(h)
	}

	return entity, nil
}

// ReadPodTemplateSpecHashes pulls the PodTemplateSpecHash that Tilt injected
// into this entity's metadata during deploy (if any)
func ReadPodTemplateSpecHashes(entity K8sEntity) ([]PodTemplateSpecHash, error) {
	templateSpecs, err := ExtractPodTemplateSpec(&entity)
	if err != nil {
		return nil, err
	}

	var ret []PodTemplateSpecHash
	for _, ts := range templateSpecs {
		ret = append(ret, PodTemplateSpecHash(ts.Labels[TiltPodTemplateHashLabel]))
	}

	return ret, nil
}
