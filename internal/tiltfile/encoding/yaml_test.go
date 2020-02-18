package encoding

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/tiltfile/io"
)

func TestReadYAML(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	var document = `
key1: foo
key2:
    key3: "bar"
    key4: true
key5: 3
`
	f.File("options.yaml", document)
	f.File("Tiltfile", `
observed = read_yaml("options.yaml")

expected = {
  'key1': 'foo',
  'key2': {
    'key3': 'bar',
    'key4': True
  },
  'key5': 3,
}

load('assert.tilt', 'assert')
assert.equals(expected, observed)
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

func TestReadYAMLDefaultValue(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
result = read_yaml("dne.yaml", "hello")

load('assert.tilt', 'assert')
assert.equals('hello', result)
`)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)
}

func TestReadYAMLStreamDefaultValue(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
result = read_yaml_stream("dne.yaml", ["hello", "goodbye"])

load('assert.tilt', 'assert')
assert.equals(['hello', 'goodbye'], result)
`)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)
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

func TestDecodeYAMLDocument(t *testing.T) {
	for _, blob := range []bool{false, true} {
		t.Run(fmt.Sprintf("blob: %v", blob), func(t *testing.T) {
			f := newFixture(t)
			defer f.TearDown()

			d := `'- "foo"\n- baz:\n  - "bar"\n  - ""\n  - 1\n  - 2'`
			if blob {
				d = fmt.Sprintf("blob(%s)", d)
			}
			d = fmt.Sprintf("observed = decode_yaml(%s)", d)
			tf := d + `
expected = [
  "foo",
  {
    "baz": [ "bar", "", 1, 2],
  }
]

load('assert.tilt', 'assert')
assert.equals(expected, observed)
`
			f.File("Tiltfile", tf)

			_, err := f.ExecFile("Tiltfile")
			if err != nil {
				fmt.Println(f.PrintOutput())
			}
			require.NoError(t, err)
		})
	}
}

func TestDecodeYAMLEmptyString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	tf := `
observed = decode_yaml('')
expected = None

load('assert.tilt', 'assert')
assert.equals(expected, observed)
`
	f.File("Tiltfile", tf)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)
}

const yamlStream = `- foo
- baz:
  - bar
  - ""
  - 1
  - 2
---
quu:
- qux
- a:
  - 3
`

const yamlStreamAsStarlark = `[
  [
    "foo",
    {
      "baz": [ "bar", "", 1, 2],
    }
  ],
  {
    "quu": [
      "qux",
      {
		"a": [3],
      }
    ]
  },
]`

func TestReadYAMLStream(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.UseRealFS()

	f.File("test.yaml", yamlStream)
	d := "observed = read_yaml_stream('test.yaml')\n"
	d += fmt.Sprintf("expected = %s\n", yamlStreamAsStarlark)
	tf := d + `
def test():
	if expected != observed:
		print('expected: %s' % (expected))
		print('observed: %s' % (observed))
		fail()

test()

`
	f.File("Tiltfile", tf)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)
}

// call read_yaml on a stream, get an error
func TestReadYAMLUnexpectedStream(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.UseRealFS()

	f.File("test.yaml", yamlStream)
	tf := "observed = read_yaml('test.yaml')\n"
	f.File("Tiltfile", tf)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected a yaml document but found a yaml stream")
}

func TestDecodeYAMLStream(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	d := yamlStream
	d = fmt.Sprintf("observed = decode_yaml_stream('''%s''')\n", d)
	d += fmt.Sprintf("expected = %s\n", yamlStreamAsStarlark)
	tf := d + `
load('assert.tilt', 'assert')
assert.equals(expected, observed)

`
	f.File("Tiltfile", tf)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)
}

func TestDecodeYAMLUnexpectedStream(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	tf := fmt.Sprintf("observed = decode_yaml('''%s''')\n", yamlStream)
	f.File("Tiltfile", tf)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected a yaml document but found a yaml stream")
}

func TestEncodeYAML(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
expected = '''key1: foo
key2:
  key3: bar
  key4: true
key5: 3
key6:
- foo
- 7
'''
observed = encode_yaml({
  'key1': 'foo',
  'key2': {
    'key3': 'bar',
    'key4': True
  },
  'key5': 3,
  'key6': [
    'foo',
    7,
  ]
})

load('assert.tilt', 'assert')
assert.equals(expected, str(observed))
`)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)
}

func TestEncodeYAMLStream(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	tf := fmt.Sprintf("expected = '''%s'''\n", yamlStream)
	tf += fmt.Sprintf("observed = encode_yaml_stream(%s)\n", yamlStreamAsStarlark)
	tf += `
load('assert.tilt', 'assert')
assert.equals(expected, str(observed))
`

	f.File("Tiltfile", tf)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)
}

func TestEncodeYAMLNonStringMapKey(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `encode_yaml({1: 'hello'})`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "only string keys are supported in maps. found key '1' of type int64")
}

func TestEncodeYAMLNonYAMLable(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
encode_yaml(blob('hello'))
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported type io.Blob")
}
