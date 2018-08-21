package model

import "testing"

func TestEscapingEntrypoint(t *testing.T) {
	cmd := ToShellCmd("echo \"hi\"")
	actual := cmd.EntrypointStr()
	expected := `ENTRYPOINT ["sh", "-c", "echo \"hi\""]`
	if actual != expected {
		t.Fatalf("expected %q, actual %q", expected, actual)
	}
}

func TestEscapingRun(t *testing.T) {
	cmd := ToShellCmd("echo \"hi\"")
	actual := cmd.RunStr()
	expected := `RUN ["sh", "-c", "echo \"hi\""]`
	if actual != expected {
		t.Fatalf("expected %q, actual %q", expected, actual)
	}
}
