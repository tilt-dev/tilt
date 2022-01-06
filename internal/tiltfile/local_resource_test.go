package tiltfile

import "testing"

func TestTestFnDeprecated(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
test("test", "echo hi")
`)
	f.loadAssertWarnings(testDeprecationMsg)
}
