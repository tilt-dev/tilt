package value

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func stringSeq(v ...string) starlark.Sequence {
	var values []starlark.Value
	for _, x := range v {
		values = append(values, starlark.String(x))
	}
	return starlark.NewList(values)
}

func TestStringSequence(t *testing.T) {
	type tc struct {
		input    starlark.Value
		expected []string
		err      string
	}
	tcs := []tc{
		{nil, nil, ""},
		{stringSeq(), nil, ""},
		{starlark.NewList([]starlark.Value{}), nil, ""},
		{stringSeq("abc123", "def456"), []string{"abc123", "def456"}, ""},
		{starlark.NewList([]starlark.Value{starlark.MakeInt(35)}), nil, "'35' is a starlark.Int, not a string"},
	}
	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// underlying, StringSequence.Unpack() uses SequenceToStringSlice(); however, since the latter is also
			// exported, it's also tested explicitly here to ensure consistent behavior between the two
			var v StringSequence
			err := v.Unpack(tc.input)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				assert.Equal(t, tc.expected, []string(v))
			}

			inputSeq, ok := tc.input.(starlark.Sequence)
			if ok {
				v, err = SequenceToStringSlice(inputSeq)
				if tc.err != "" {
					require.EqualError(t, err, tc.err)
				} else {
					assert.Equal(t, tc.expected, []string(v))
					// test the round-trip (note: we have to test with iterators as direct
					// equality can't be guaranteed due to difference in semantics around
					// empty vs nil slices)
					expectedSeq := tc.input.(starlark.Sequence)
					actualSeq := v.Sequence()
					if assert.Equal(t, expectedSeq.Len(), actualSeq.Len()) {
						expectedIt := expectedSeq.Iterate()
						actualIt := v.Sequence().Iterate()
						var expectedVal starlark.Value
						for expectedIt.Next(&expectedVal) {
							var actualVal starlark.Value
							require.True(t, actualIt.Next(&actualVal))
							assert.Equal(t, expectedVal, actualVal)
						}
					}
				}
			}
		})
	}
}

func TestInt32Value_Unpack(t *testing.T) {
	type tc struct {
		input    starlark.Value
		expected int32
		err      string
	}
	tcs := []tc{
		{nil, 0, "got NoneType, want int"},
		{starlark.MakeInt(0), 0, ""},
		{starlark.MakeInt(-123), -123, ""},
		{starlark.MakeInt(456), 456, ""},
		{starlark.MakeInt64(math.MaxInt32 + 1), 0, "value out of range for int32: 2147483648"},
		{starlark.MakeInt64(math.MinInt32 - 1), 0, "value out of range for int32: -2147483649"},
	}
	for _, tc := range tcs {
		var name string
		if tc.input != nil {
			name = tc.input.String()
		} else {
			name = "nil"
		}

		t.Run(name, func(t *testing.T) {
			var v Int32
			err := v.Unpack(tc.input)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				assert.Equal(t, tc.expected, v.Int32())
			}
		})
	}
}
