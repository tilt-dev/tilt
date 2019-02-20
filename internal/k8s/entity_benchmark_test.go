package k8s

import (
	"testing"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/yaml"
)

func BenchmarkParseUnparseSingle(b *testing.B) {
	run := func() {
		entities, err := ParseYAMLFromString(testyaml.PodYAML)
		if err != nil {
			b.Fatal(err)
		}
		_, err = SerializeYAML(entities)
		if err != nil {
			b.Fatal(err)
		}
	}
	for i := 0; i < b.N; i++ {
		run()
	}
}

func BenchmarkParseUnparseLonger(b *testing.B) {
	bigYaml := makeBigYaml(25)
	run := func() {
		entities, err := ParseYAMLFromString(bigYaml)
		if err != nil {
			b.Fatal(err)
		}
		_, err = SerializeYAML(entities)
		if err != nil {
			b.Fatal(err)
		}
	}
	for i := 0; i < b.N; i++ {
		run()
	}
}

func BenchmarkParseUnparseLongest(b *testing.B) {
	bigYaml := makeBigYaml(100)

	run := func() {
		entities, err := ParseYAMLFromString(bigYaml)
		if err != nil {
			b.Fatal(err)
		}
		_, err = SerializeYAML(entities)
		if err != nil {
			b.Fatal(err)
		}
	}
	for i := 0; i < b.N; i++ {
		run()
	}
}

func makeBigYaml(n int) string {
	strs := make([]string, n)
	for i := 0; i < n; i++ {
		strs[i] = testyaml.PodYAML
	}
	return yaml.ConcatYAML(strs...)
}
