package model

import (
	"regexp"
	"strings"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func IsEmptyDockerComposeProject(p v1alpha1.DockerComposeProject) bool {
	return len(p.ConfigPaths) == 0 && p.YAML == ""
}

// normalization logic from https://github.com/compose-spec/compose-go/blob/c39f6e771fe5034fe1bec40ba5f0285ec60f5efe/cli/options.go#L366-L371
func NormalizeName(s string) string {
	r := regexp.MustCompile("[a-z0-9_-]")
	s = strings.ToLower(s)
	s = strings.Join(r.FindAllString(s, -1), "")
	return strings.TrimLeft(s, "_-")
}
