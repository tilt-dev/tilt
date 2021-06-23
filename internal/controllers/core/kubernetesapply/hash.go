package kubernetesapply

import (
	"crypto"
	"encoding/base64"
	"fmt"
	"hash"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var defaultJSONIterator = createDefaultJSONIterator()

func createDefaultJSONIterator() jsoniter.API {
	return jsoniter.Config{
		EscapeHTML:             true,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
		CaseSensitive:          true,
	}.Froze()
}

// Compute the hash of all the inputs we fed into this apply.
func ComputeInputHash(spec v1alpha1.KubernetesApplySpec, imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) (string, error) {
	w := newHashWriter()
	err := w.append(spec)
	if err != nil {
		return "", err
	}

	for _, imageMapName := range spec.ImageMaps {
		imageMap, ok := imageMaps[types.NamespacedName{Name: imageMapName}]
		if !ok {
			return "", fmt.Errorf("missing image map: %v", err)
		}
		err = w.append(imageMap.Spec)
		if err != nil {
			return "", fmt.Errorf("hashing %s spec: %v", imageMapName, err)
		}
		err = w.append(imageMap.Status)
		if err != nil {
			return "", fmt.Errorf("hashing %s status: %v", imageMapName, err)
		}
	}

	return w.done(), nil
}

type hashWriter struct {
	h hash.Hash
}

func newHashWriter() *hashWriter {
	return &hashWriter{h: crypto.SHA1.New()}
}

func (w hashWriter) append(o interface{}) error {
	data, err := defaultJSONIterator.Marshal(o)
	if err != nil {
		return errors.Wrap(err, "serializing object for hash")
	}
	_, err = w.h.Write(data)
	if err != nil {
		return errors.Wrap(err, "computing hash")
	}
	return nil
}

func (w hashWriter) done() string {
	return base64.URLEncoding.EncodeToString(w.h.Sum(nil))
}
