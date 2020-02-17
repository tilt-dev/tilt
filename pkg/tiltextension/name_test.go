package tiltextension

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var validNameTests = []struct {
	in          string
	valid       bool
	errContains string
}{
	// valid
	{"test", true, ""},
	{"some-extension", true, ""},
	{"example.com", true, ""},
	{"under_score", true, ""},
	{"test.go", true, ""},
	{"123numeric", true, ""},
	// invalid
	{"crazy!", false, `name can only contain URL-friendly characters`},
	{"", false, "name length must be greater than zero"},
	{".start-with-period", false, "name cannot start with a period"},
	{"_start-with-underscore", false, "name cannot start with an underscore"},
	{"contain:colons", false, "name cannot contain `:`"},
	{" leading-space", false, "name cannot contain leading or trailing spaces"},
	{"trailing-space ", false, "name cannot contain leading or trailing spaces"},
	{"s/l/a/s/h/e/s", false, "name can only contain URL-friendly characters"},
	{"tilt_modules", false, "tilt_modules is a banned name"},
	{"Tiltfile", false, "Tiltfile is a banned name"},
	{strings.Repeat("long", 200), false, "name cannot contain more than 214 characters"},
}

func TestValidateName(t *testing.T) {
	for _, tt := range validNameTests {
		t.Run(tt.in, func(t *testing.T) {
			result := ValidateName(tt.in)
			if tt.valid && result != nil {
				t.Errorf("Expected valid, got %v", result)
			} else if !tt.valid && result == nil {
				t.Errorf("Expected invalid, got valid")
			} else if !tt.valid && result != nil {
				assert.Contains(t, result.Error(), tt.errContains)
			}
		})
	}
}
