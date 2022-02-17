package tiltfile

import "testing"

func TestTestFnDeprecated(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
test("test", "echo hi")
`)
	f.loadAssertWarnings(testDeprecationMsg)
}
