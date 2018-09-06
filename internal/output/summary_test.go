package output

import (
	"strings"
	"testing"
)

func TestSummary(t *testing.T) {
	output := &strings.Builder{}
	s := newSummary(output)

	s.parse()
	s.print()

	expected := `Steps: 3`

	if output.String() != string(expected) {
		t.Errorf("EXPECTED:\n%s\nGOT:\n%s\n", expected, output.String())
	}
}
