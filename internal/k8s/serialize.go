package k8s

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	yamlEncoder "sigs.k8s.io/yaml"
)

func ParseYAMLFromString(yaml string) ([]K8sEntity, error) {
	buf := bytes.NewBuffer([]byte(yaml))
	return ParseYAML(buf)
}

func decodeMetaList(list *metav1.List) ([]K8sEntity, error) {
	result := make([]K8sEntity, 0, len(list.Items))
	for _, item := range list.Items {
		decoded, err := decodeRawExtension(item)
		if err != nil {
			return nil, err
		}
		result = append(result, decoded...)
	}
	return result, nil
}

func decodeList(list *v1.List) ([]K8sEntity, error) {
	return decodeMetaList((*metav1.List)(list))
}

func decodeToRuntimeObj(ext runtime.RawExtension) (runtime.Object, error) {
	ext.Raw = bytes.TrimSpace(ext.Raw)

	// NOTE(nick): I LOL'd at the null check, but it's what kubectl does.
	if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
		return nil, nil
	}

	obj, _, decodeErr :=
		scheme.Codecs.UniversalDeserializer().Decode(ext.Raw, nil, nil)
	if decodeErr == nil {
		return obj, nil
	}

	// decode as unstructured - if the _original_ decode error was due to it
	// being a non-standard type, the unstructured object will be returned;
	// otherwise, it'll be used to provide additional context to the error if
	// possible
	var unst unstructured.Unstructured
	_, gvk, err :=
		unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, &unst)
	if err != nil {
		if gvk != nil && gvk.Kind != "" {
			// add the kind if possible (decode will return it even on error
			// if it was able to parse it out first); we don't have the name
			// available since both structured + unstructured decodes failed
			decodeErr = fmt.Errorf("decoding %s object: %w", gvk.Kind, decodeErr)
		}
		// ignore the unstructured error and instead use the original decode
		// error, as it's more likely to be descriptive
		return nil, decodeErr
	}
	obj = &unst

	if runtime.IsNotRegisteredError(decodeErr) {
		// not a built-in/known K8s type, but a valid apiserver object, so
		// return the unstructured object
		return obj, nil
	}

	kind := unst.GetKind()
	if kind == "" {
		kind = "Kubernetes object"
	}
	// add the kind and object name to the error
	// example -> decoding Secret "foo": illegal base64 data at input byte 0
	err = fmt.Errorf("decoding %s %q: %w", kind, unst.GetName(), decodeErr)
	return nil, err
}

func decodeRawExtension(ext runtime.RawExtension) ([]K8sEntity, error) {
	obj, err := decodeToRuntimeObj(ext)
	if err != nil {
		return nil, err
	} else if obj == nil {
		return nil, nil
	}

	// Check to see if this is a list, and we can decode the list elements.
	list, isList := obj.(*v1.List)
	if isList {
		return decodeList(list)
	}

	metaList, isMetaList := obj.(*metav1.List)
	if isMetaList {
		return decodeMetaList(metaList)
	}

	return []K8sEntity{NewK8sEntity(obj)}, nil
}

// Parse the YAML into entities.
// Loosely based on
// https://github.com/kubernetes/cli-runtime/blob/d6a36215b15f83b94578f2ffce5d00447972e8ae/pkg/genericclioptions/resource/visitor.go#L583
func ParseYAML(k8sYaml io.Reader) ([]K8sEntity, error) {
	reader := bufio.NewReader(k8sYaml)
	decoder := yamlDecoder.NewYAMLOrJSONDecoder(reader, 4096)

	result := make([]K8sEntity, 0)
	for {
		ext := runtime.RawExtension{}
		if err := decoder.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		entities, err := decodeRawExtension(ext)
		if err != nil {
			return nil, err
		}
		result = append(result, entities...)
	}

	return result, nil
}

// Serializes the provided K8s object as YAML to the given writer.
//
// By convention, all K8s objects contain ObjectMetadata, Spec, and Status.
// This only serializes the metadata and spec, skipping the status.
func serializeSpec(obj runtime.Object, w io.Writer) error {
	json, err := specJSONIterator.Marshal(obj)
	if err != nil {
		return err
	}
	data, err := yamlEncoder.JSONToYAML(json)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// Serializes the provided K8s objects as YAML.
//
// By convention, all K8s objects contain ObjectMetadata, Spec, and Status.
// This only serializes the metadata and spec, skipping the status.
func SerializeSpecYAML(decoded []K8sEntity) (string, error) {
	buf, err := SerializeSpecYAMLToBuffer(decoded)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func SerializeSpecYAMLToBuffer(decoded []K8sEntity) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)
	for i, obj := range decoded {
		if i != 0 {
			buf.Write([]byte("\n---\n"))
		}
		err := serializeSpec(obj.Obj, buf)
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}
