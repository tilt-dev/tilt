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
	cmd := ToShellCmd("echo \"hi\"")
	actual := cmd.RunStr()
	expected := `RUN echo "hi"`
	if actual != expected {
		t.Fatalf("expected %q, actual %q", expected, actual)
	}
}

func TestHash(t *testing.T) {
	service := Service{
		DockerfileText: "FROM alpine",
	}
	r1, err := service.Hash()
	if err != nil {
		t.Fatal(err)
	}
	service2 := Service{
		DockerfileText: "FROM alpine",
	}
	r2, err := service2.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if r1 != r2 {
		t.Errorf("Expected %d to equal %d", r1, r2)
	}
	service3 := Service{
		DockerfileText: "FROM alpine2",
	}

	r3, err := service3.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if r3 == r2 {
		t.Errorf("Expected %d to NOT equal %d", r3, r2)
	}

	mounts1 := []Mount{
		Mount{
			Repo: LocalGithubRepo{
				LocalPath: "/hi",
			},
		},
	}
	service4 := Service{
		DockerfileText: "FROM alpine",
		Mounts:         mounts1,
	}
	r4, err := service4.Hash()
	if err != nil {
		t.Fatal(err)
	}

	mounts2 := []Mount{
		Mount{
			Repo: LocalGithubRepo{
				LocalPath: "/hello",
			},
		},
	}
	service5 := Service{
		DockerfileText: "FROM alpine",
		Mounts:         mounts2,
	}
	r5, err := service5.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if r4 == r5 {
		t.Errorf("Expected %d to NOT equal %d", r3, r2)
	}
}
