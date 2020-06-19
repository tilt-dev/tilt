//+build integration

package integration

import (
	"testing"
)

func TestCIAutoInitFalse(t *testing.T) {
	f := newK8sFixture(t, "ci_auto_init_false")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltCI()
}
