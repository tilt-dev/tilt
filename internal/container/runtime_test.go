package container

import "testing"

type expectedRuntime struct {
	expected Runtime
	string
}

func TestRuntimeFromVersionString(t *testing.T) {
	table := []expectedRuntime{
		{RuntimeDocker, "docker://18.6.1"},
		{RuntimeCrio, "cri-o://1.13.0"},
		{RuntimeContainerd, "containerd://Unknown"},
		{RuntimeUnknown, "garbage"},
		{RuntimeUnknown, "garbage::moregarbage"},
		{RuntimeUnknown, "garbage:moregarbage:evenmoregarbage"},
	}

	for _, tt := range table {
		t.Run(tt.string, func(t *testing.T) {
			actual := RuntimeFromVersionString(tt.string)
			if actual != tt.expected {
				t.Errorf("Expected %s, actual %s", tt.expected, actual)
			}
		})
	}
}
