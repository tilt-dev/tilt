//+build integration

package integration

import "testing"

func TestOneUpCustom(t *testing.T) {
	f := newFixture(t, "oneup_custom")
	defer f.TearDown()
}
