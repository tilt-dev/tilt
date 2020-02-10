package encoding

import (
	"fmt"
	"testing"

	"github.com/windmilleng/tilt/internal/tiltfile/io"

	"github.com/stretchr/testify/require"
)

func TestReadYAML(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.UseRealFS()

	var document = `
key1: foo
key2:
    key3: "bar"
    key4: true
key5: 3
`
	f.File("options.yaml", document)
	f.File("Tiltfile", `
result = read_yaml("options.yaml")

expected = {
  'key1': 'foo',
  'key2': {
    'key3': 'bar',
    'key4': True
  },
  'key5': 3,
}

def test():
	if expected != result:
		print('expected: %s' % (expected))
		print('observed: %s' % (result))
		fail()

test()
`)

	result, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)

	rs, err := io.GetState(result)
	require.NoError(t, err)
	require.Contains(t, rs.Files, f.JoinPath("options.yaml"))
}

func TestYAMLDoesNotExist(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `result = read_yaml("dne.yaml")`)
	result, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "dne.yaml: no such file or directory")

	rs, err := io.GetState(result)
	require.NoError(t, err)
	require.Contains(t, rs.Files, f.JoinPath("dne.yaml"))
}

func TestMalformedYAML(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.UseRealFS()

	var document = `
key1: foo
key2:
    key3: "bar
    key4: true
key5: 3
`
	f.File("options.yaml", document)

	f.File("Tiltfile", `result = read_yaml("options.yaml")`)
	result, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "error parsing YAML from options.yaml: error converting YAML to JSON: yaml: line 7: found unexpected end of stream")

	rs, err := io.GetState(result)
	require.NoError(t, err)
	require.Contains(t, rs.Files, f.JoinPath("options.yaml"))

}

func TestDecodeYAML(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
observed = decode_yaml('- "foo"\n- baz:\n  - "bar"\n  - ""\n  - 1\n  - 2')
expected = [
  "foo",
  {
    "baz": [ "bar", "", 1, 2],
  }
]

def test():
	if expected != observed:
		print('expected: %s' % (expected))
		print('observed: %s' % (observed))
		fail()

test()

`)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)
}
