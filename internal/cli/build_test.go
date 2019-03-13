package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvideWebVersion(t *testing.T) {
	assert.Equal(t, fmt.Sprintf("v%s", devVersion), string(provideWebVersion()))
}
