package tiltextension

// Most of the code in this file is adopted from NPM's module name rules
// https://github.com/npm/validate-npm-package-name/blob/master/index.js

import (
	"fmt"
	"net/url"
	"strings"
)

const maxModuleNameLength = 214

var banList = []string{
	"tilt_modules",
	"Tiltfile",
}

func ValidateName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("name length must be greater than zero")
	}

	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("name cannot start with a period")
	}

	if strings.HasPrefix(name, "_") {
		return fmt.Errorf("name cannot start with an underscore")
	}

	if strings.TrimSpace(name) != name {
		return fmt.Errorf("name cannot contain leading or trailing spaces")
	}

	for _, b := range banList {
		if strings.EqualFold(name, b) {
			return fmt.Errorf("%s is a banned name", b)
		}
	}

	if len(name) > maxModuleNameLength {
		return fmt.Errorf("name cannot contain more than 214 characters")
	}

	if url.PathEscape(name) != name {
		return fmt.Errorf("name can only contain URL-friendly characters")
	}

	if strings.Contains(name, ":") {
		return fmt.Errorf("name cannot contain `:`")
	}

	return nil
}
