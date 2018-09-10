package model

import (
	"testing"
)

func TestMatches(t *testing.T) {
	root := "/Users/mary/go/src/github.com/windmilleng/blorg"
	path := "node_modules"

	pm, err := NewPathMatcher(root, path)
	if err != nil {
		t.Fatal(err)
	}

	changedFile := "/Users/mary/go/src/github.com/windmilleng/blorg/node_modules"

	match, err := pm.Matches(changedFile, true)
	if err != nil {
		t.Fatal(err)
	}
	if match == false {
		t.Errorf("Expected %s to match %v, but it didn't", changedFile, pm)
	}
}

func TestNoMatch(t *testing.T) {
	root := "/Users/mary/go/src/github.com/windmilleng/blorg"
	path := "node_modules"

	pm, err := NewPathMatcher(root, path)
	if err != nil {
		t.Fatal(err)
	}

	changedFile := "/Users/mary/go/src/github.com/windmilleng/blorg/blorg_modules"

	match, err := pm.Matches(changedFile, true)
	if err != nil {
		t.Fatal(err)
	}
	if match == true {
		t.Errorf("Expected %s to not match %v, but it did", changedFile, pm)
	}
}
