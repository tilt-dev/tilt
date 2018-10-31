package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	noSep = `this: is
some: yaml`
	endSep = `iLoveYaml: meh
betterThanXml: true
---`
	startSep = `---
someMap:
    stuff: yes
    things: also yes
`
)

func TestConcatYamlNoSeparators(t *testing.T) {
	expected := `---
someMap:
    stuff: yes
    things: also yes
---
this: is
some: yaml`
	assert.Equal(t, expected, ConcatYAML(startSep, noSep))
}

func TestConcatYamlBothSeparators(t *testing.T) {
	expected := `iLoveYaml: meh
betterThanXml: true
---
someMap:
    stuff: yes
    things: also yes`
	assert.Equal(t, expected, ConcatYAML(endSep, startSep))
}

func TestConcatYamlEndSep(t *testing.T) {
	expected := `iLoveYaml: meh
betterThanXml: true
---
this: is
some: yaml`
	assert.Equal(t, expected, ConcatYAML(endSep, noSep))
}

func TestConcatYamlStartSep(t *testing.T) {
	expected := `---
someMap:
    stuff: yes
    things: also yes
---
someMap:
    stuff: yes
    things: also yes`
	assert.Equal(t, expected, ConcatYAML(startSep, startSep))
}

func TestConcatManyYamls(t *testing.T) {
	expected := `---
someMap:
    stuff: yes
    things: also yes
---
this: is
some: yaml
---
iLoveYaml: meh
betterThanXml: true
---`

	assert.Equal(t, expected, ConcatYAML(startSep, noSep, endSep))
}

func TestNoopConcatYaml(t *testing.T) {
	assert.Equal(t, "", ConcatYAML())

	assert.Equal(t, noSep, ConcatYAML(noSep))
}
