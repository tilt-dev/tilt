package model

import (
	"testing"
)

func TestEscapingEntrypoint(t *testing.T) {
	cmd := Cmd{Argv: []string{"bash", "-c", "echo \"hi\""}}
	actual := cmd.EntrypointStr()
	expected := `ENTRYPOINT ["bash", "-c", "echo \"hi\""]`
	if actual != expected {
		t.Fatalf("expected %q, actual %q", expected, actual)
	}
}

func TestEscapingRun(t *testing.T) {
	cmd := Cmd{Argv: []string{"bash", "-c", "echo \"hi\""}}
	actual := cmd.RunStr()
	expected := `RUN ["bash", "-c", "echo \"hi\""]`
	if actual != expected {
		t.Fatalf("expected %q, actual %q", expected, actual)
	}
}

func TestNormalFormRun(t *testing.T) {
	cmd := ToUnixCmd("echo \"hi\"")
	actual := cmd.RunStr()
	expected := `RUN echo "hi"`
	if actual != expected {
		t.Fatalf("expected %q, actual %q", expected, actual)
	}
}
