package encoding

import (
	"fmt"
	"testing"

	"github.com/windmilleng/tilt/internal/tiltfile/io"

	"github.com/stretchr/testify/require"
)

func TestReadJSON(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.UseRealFS()

	var document = `{
	  "key1": "foo",
	  "key2": {
	    "key3": "bar",
	    "key4": true
	  },
      "key5": 3
	}
	`
	f.File("options.json", document)
	f.File("Tiltfile", `
result = read_json("options.json")

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
	require.Contains(t, rs.Files, f.JoinPath("options.json"))
}

func TestJSONDoesNotExist(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `result = read_json("dne.json")`)
	result, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "dne.json: no such file or directory")

	rs, err := io.GetState(result)
	require.NoError(t, err)
	require.Contains(t, rs.Files, f.JoinPath("dne.json"))
}

func TestMalformedJSON(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.UseRealFS()

	f.File("options.json", `["foo", {"baz":["bar", "", 1, 2]}`)

	f.File("Tiltfile", `result = read_json("options.json")`)
	result, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "error parsing JSON from options.json: unexpected end of JSON input")

	rs, err := io.GetState(result)
	require.NoError(t, err)
	require.Contains(t, rs.Files, f.JoinPath("options.json"))
}

func TestDecodeJSON(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	for _, blob := range []bool{false, true} {
		t.Run(fmt.Sprintf("blob: %v", blob), func(t *testing.T) {
			d := `'["foo", {"baz":["bar", "", 1, 2]}]'`
			if blob {
				d = fmt.Sprintf("blob(%s)", d)
			}
			d = fmt.Sprintf("observed = decode_json(%s)", d)
			tf := d + `
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

func TestEncodeJSON(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
expected = '''[
  "foo",
  {
    "baz": [
      "bar",
      "",
      1,
      2
    ]
  }
]
'''
observed = encode_json([
  "foo",
  {
    "baz": [ "bar", "", 1, 2],
  }
])

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

func TestEncodeJSONNonStringMapKey(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `encode_json({1: 'hello'})`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "only string keys are supported in maps. found key '1' of type int64")
}

func TestEncodeJSONNonJSONable(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
encode_json(blob('hello'))
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported type io.Blob")
}
