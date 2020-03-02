package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildReasonString(t *testing.T) {
	assert.Equal(t, "Changed Files | Config Changed", BuildReasonFlagChangedFiles.With(BuildReasonFlagConfig).String())
	assert.Equal(t, "Web Trigger", BuildReasonFlagInit.With(BuildReasonFlagTriggerWeb).String())
}
