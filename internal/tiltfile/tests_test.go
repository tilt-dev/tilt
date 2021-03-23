package tiltfile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/model"
)

func TestTestWithDefaults(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()

	f.file("Tiltfile", `
test("test-foo", "echo hi")
`)

	f.load()

	foo := f.assertNextManifest("test-foo", localTarget(updateCmd(f.Path(), "echo hi", nil)))
	assert.True(t, foo.LocalTarget().IsTest, "should be flagged as test manifest")
}

func TestTestWithExplicit(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()

	f.file("Tiltfile", `
t = test("test-foo", "echo hi",
		deps=["a.txt", "b.txt"],
		tags=["beep", "boop"],
		trigger_mode=TRIGGER_MODE_MANUAL,
)
`)

	f.load()

	foo := f.assertNextManifest("test-foo", localTarget(
		updateCmd(f.Path(), "echo hi", nil),
		deps("a.txt", "b.txt")))
	assert.True(t, foo.LocalTarget().IsTest, "should be flagged as test manifest")
	assert.Equal(t, []string{"beep", "boop"}, foo.LocalTarget().Tags)
	assert.Equal(t, model.TriggerModeManualWithAutoInit, foo.TriggerMode)
}

// TODO:
//   - timeout (once implemented)
//   - allowParallel defaults to true but can be set to false
