package k8s

import (
	"bufio"
	"bytes"
	"io"

	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

func ParseYAMLFromString(yaml string) ([]K8sEntity, error) {
	buf := bytes.NewBuffer([]byte(yaml))
	return ParseYAML(buf)
}

func ParseYAML(k8sYaml io.Reader) ([]K8sEntity, error) {
	reader := bufio.NewReader(k8sYaml)
	yamlReader := yaml.NewYAMLReader(reader)

	result := make([]K8sEntity, 0)
	for true {
		yamlPart, err := yamlReader.Read()
		if err != nil && err != io.EOF {
			return nil, err
		}

		if err == io.EOF {
			return result, nil
		}

		deserializer := scheme.Codecs.UniversalDeserializer()
		obj, groupVersionKind, err := deserializer.Decode(yamlPart, nil, nil)
		if err != nil {
			return nil, err
		}

		result = append(result, K8sEntity{
			Obj:  obj,
			Kind: groupVersionKind,
		})
	}

	return nil, nil
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
