package model

import (
	"fmt"
	"regexp"
)

// If there are multiple tilt apiservers running,
// we can refer to them by name.
type APIServerName string

var apiServerNameMatcher = regexp.MustCompile("^[a-z0-9-]+$")

// Makes sure the apiserver name is well-formed.
func ValidateAPIServerName(name APIServerName) error {
	if !apiServerNameMatcher.MatchString(string(name)) {
		return fmt.Errorf("malformed name, must match regexp /[a-z0-9-]+/. Actual: %s", name)
	}
	return nil
}

// Each apiserver has a name based on the web port.
func DefaultAPIServerName(port WebPort) APIServerName {
	if port == DefaultWebPort {
		return "tilt-default"
	}
	return APIServerName(fmt.Sprintf("tilt-%d", port))
}

// Determines what the API server name should be
// based on the --port flag.
//
// TODO(nick): Long-term, most tools in this space are moving
// away from making users manage ports, and using names
// to identify different instances.
func ProvideAPIServerName(port WebPort) APIServerName {
	return DefaultAPIServerName(port)
}
