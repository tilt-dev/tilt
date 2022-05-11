package encoding

import (
	"fmt"
	"testing"

	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/tiltfile/io"

	"github.com/stretchr/testify/require"
)

func TestReadJSON(t *testing.T) {
	f := newFixture(t)

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
	require.Contains(t, rs.Paths, f.JoinPath("options.json"))
}

func TestJSONDoesNotExist(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `result = read_json("dne.json")`)
	result, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "dne.json")
	testutils.AssertIsNotExist(t, err)

	rs, err := io.GetState(result)
	require.NoError(t, err)
	require.Contains(t, rs.Paths, f.JoinPath("dne.json"))
}

func TestMalformedJSON(t *testing.T) {
	f := newFixture(t)

	f.UseRealFS()

	f.File("options.json", `["foo", {"baz":["bar", "", 1, 2]}`)

	f.File("Tiltfile", `result = read_json("options.json")`)
	result, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "error parsing JSON from")
	require.Contains(t, err.Error(), "options.json: unexpected EOF")

	rs, err := io.GetState(result)
	require.NoError(t, err)
	require.Contains(t, rs.Paths, f.JoinPath("options.json"))
}

func TestDecodeJSON(t *testing.T) {
	f := newFixture(t)

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

	f.File("Tiltfile", `encode_json({1: 'hello'})`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "only string keys are supported in maps. found key '1' of type int64")
}

func TestEncodeJSONNonJSONable(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
encode_json(blob('hello'))
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported type io.Blob")
}

func TestDecodeJSONIntFloat(t *testing.T) {
	f := newFixture(t)
	f.File("Tiltfile", `
json = '{"int":42,"float":3.14,"intfloat":3.0}'
x = decode_json(json)
if repr(x["int"]) != "42":
  fail('repr(int) value was not "42": ' + repr(x["int"]))
if repr(x["float"]) != "3.14":
  fail('repr(float) value was not a "3.14": ' + repr(x["float"]))
if repr(x["intfloat"]) != "3.0":
  fail('repr(intfloat) value was not a "3.0": ' + repr(x["intfloat"]))
`)

	_, err := f.ExecFile("Tiltfile")
	if err != nil {
		fmt.Println(f.PrintOutput())
	}
	require.NoError(t, err)
}

func TestDecodeInvalidJSONMultipleValues(t *testing.T) {
	f := newFixture(t)
	f.File("stream.json", `{"a":1,"b":2}
{"a":2,"b":3}`)
	f.File("Tiltfile", `read_json("stream.json")`)
	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiple JSON values")
}
