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
		{stringSeq(), []string{}, ""},
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
		{starlark.MakeInt64(math.MaxInt32 + 1), 0, "2147483648 out of range"},
	}
	for _, tc := range tcs {
		var name string
		if tc.input != nil {
			name = tc.input.String()
		} else {
			name = "nil"
		}

		t.Run(name, func(t *testing.T) {
			var v Int32Value
			err := v.Unpack(tc.input)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				assert.Equal(t, tc.expected, int32(v))
			}
		})
	}
}
