package apis_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/apis"
)

func TestSanitizeName(t *testing.T) {
	testCases := [][2]string{
		{"abc123", "abc123"},
		{"_def%456_", "_def_456_"},
		{"$/./resource", "$_._resource"},
	}

	for _, tc := range testCases {
		t.Run(tc[0], func(t *testing.T) {
			assert.Equal(t, tc[1], apis.SanitizeName(tc[0]))
		})
	}

	t.Run("Max Length Exceeded", func(t *testing.T) {
		tooLong := strings.Repeat("abc123", 50)
		expected := tooLong[:apis.MaxNameLength-9] + "-3c8e47c5"
		assert.Equal(t, expected, apis.SanitizeName(tooLong))
	})
}
