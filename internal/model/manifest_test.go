package model

import (
	"testing"
)

type truePathMatcherStruct struct{}

func (truePathMatcherStruct) Matches(f string, isDir bool) (bool, error) {
	return true, nil
}

type falsePathMatcherStruct struct{}

func (falsePathMatcherStruct) Matches(f string, isDir bool) (bool, error) {
	return false, nil
}

var equalitytests = []struct {
	m1       Manifest
	m2       Manifest
	expected bool
}{
	{
		Manifest{},
		Manifest{},
		true,
	},
	{
		Manifest{},
		Manifest{
			BaseDockerfile: "FROM node",
		},
		false,
	},
	{
		Manifest{
			BaseDockerfile: "FROM node",
		},
		Manifest{
			BaseDockerfile: "FROM node",
		},
		true,
	},
	{
		Manifest{
			BaseDockerfile: "FROM node",
			FileFilter:     truePathMatcherStruct{},
		},
		Manifest{
			BaseDockerfile: "FROM node",
			FileFilter:     truePathMatcherStruct{},
		},
		true,
	},
	{
		Manifest{
			BaseDockerfile: "FROM node",
			FileFilter:     truePathMatcherStruct{},
		},
		Manifest{
			BaseDockerfile: "FROM node",
			FileFilter:     falsePathMatcherStruct{},
		},
		false,
	},
	{
		Manifest{
			Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
		},
		Manifest{
			Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
		},
		true,
	},
	{
		Manifest{
			Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
		},
		Manifest{
			Entrypoint: Cmd{Argv: []string{"bash", "-c", "echo hi"}},
		},
		false,
	},
}

func TestManifestEquality(t *testing.T) {
	for i, c := range equalitytests {
		actual := c.m1.Equal(c.m2)

		if actual != c.expected {
			t.Errorf("Test case #%d: Expected %v == %v to be %t, but got %t", i, c.m1, c.m2, c.expected, actual)
		}
	}
}
