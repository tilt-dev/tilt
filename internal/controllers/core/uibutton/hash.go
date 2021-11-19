package uibutton

import (
	"crypto"
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

const annotationSpecHash = "uibuttonspec-hash"

var defaultJSONIterator = createDefaultJSONIterator()

func createDefaultJSONConfig() jsoniter.Config {
	return jsoniter.Config{
		EscapeHTML:             true,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
		CaseSensitive:          true,
	}
}

func createDefaultJSONIterator() jsoniter.API {
	return createDefaultJSONConfig().Froze()
}

func hashUIButtonSpec(spec v1alpha1.UIButtonSpec) (string, error) {
	data, err := defaultJSONIterator.Marshal(spec)
	if err != nil {
		return "", errors.Wrap(err, "serializing spec to json")
	}

	h := crypto.SHA1.New()
	_, err = h.Write(data)
	if err != nil {
		return "", errors.Wrap(err, "writing to hash")
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:10]), nil
}
