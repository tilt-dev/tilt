package dockercomposeservice

import (
	"crypto"
	"encoding/base64"
	"hash"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"

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

// Compute the hash of a dockercompose project.
func hashProject(p v1alpha1.DockerComposeProject) (string, error) {
	w := newHashWriter()
	err := w.append(p)
	if err != nil {
		return "", err
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
