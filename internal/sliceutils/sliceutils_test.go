package sliceutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendWithoutDupesNoDupes(t *testing.T) {
	a := []string{"a", "b"}
	b := []string{"c", "d"}
	observed := AppendWithoutDupes(a, b...)
	expected := []string{"a", "b", "c", "d"}
	assert.Equal(t, expected, observed)
}

func TestAppendWithoutDupesHasADupe(t *testing.T) {
	a := []string{"a", "b"}
	b := []string{"c", "b"}
	observed := AppendWithoutDupes(a, b...)
	expected := []string{"a", "b", "c"}
	assert.Equal(t, expected, observed)
}

func TestAppendWithoutDupesEmptyA(t *testing.T) {
	a := []string{}
	b := []string{"c", "b"}
	observed := AppendWithoutDupes(a, b...)
	expected := []string{"c", "b"}
	assert.Equal(t, expected, observed)
}

func TestAppendWithoutDupesEmptyB(t *testing.T) {
	a := []string{"a", "b"}
	b := []string{}
	observed := AppendWithoutDupes(a, b...)
	expected := []string{"a", "b"}
	assert.Equal(t, expected, observed)
}
