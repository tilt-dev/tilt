package k8s

import (
	"bufio"
	"bytes"
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

func ParseYAMLFromString(yaml string) ([]K8sEntity, error) {
	buf := bytes.NewBuffer([]byte(yaml))
	return ParseYAML(buf)
}

// Parse the YAML into entities.
// Loosely based on
// https://github.com/kubernetes/cli-runtime/blob/d6a36215b15f83b94578f2ffce5d00447972e8ae/pkg/genericclioptions/resource/visitor.go#L583
func ParseYAML(k8sYaml io.Reader) ([]K8sEntity, error) {
	reader := bufio.NewReader(k8sYaml)
	decoder := yaml.NewYAMLOrJSONDecoder(reader, 4096)

	result := make([]K8sEntity, 0)
	for {
		ext := runtime.RawExtension{}
		if err := decoder.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		ext.Raw = bytes.TrimSpace(ext.Raw)

		// NOTE(nick): I LOL'd at the null check, but it's what kubectl does.
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}

		deserializer := scheme.Codecs.UniversalDeserializer()
		obj, groupVersionKind, err := deserializer.Decode(ext.Raw, nil, nil)
		if err != nil {
			return nil, err
		}

		result = append(result, K8sEntity{
			Obj:  obj,
			Kind: groupVersionKind,
		})
	}

	return result, nil
}

func SerializeYAML(decoded []K8sEntity) (string, error) {
	yamlSerializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	buf := bytes.NewBuffer(nil)
	for i, obj := range decoded {
		if i != 0 {
			buf.Write([]byte("\n---\n"))
		}
		err := yamlSerializer.Encode(obj.Obj, buf)
		if err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}
