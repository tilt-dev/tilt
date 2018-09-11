package model

import (
	"os"
	"testing"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/stretchr/testify/assert"
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

type boilTestCase struct {
	steps        []Step
	filesChanged []string
	expected     []Cmd
}

type falseMatcher struct {
	trigger string
}

func (s falseMatcher) Matches(f string, isDir bool) (bool, error) {
	return false, nil
}

type trueMatcher struct {
	trigger string
}

func (s trueMatcher) Matches(f string, isDir bool) (bool, error) {
	return true, nil
}

func TestBoilSteps(t *testing.T) {
	boilStepsTests := []boilTestCase{
		// no triggers, files changed
		boilTestCase{
			steps: []Step{
				Step{
					Cmd: ToShellCmd("echo hello"),
				},
			},
			filesChanged: []string{"foo"},
			expected: []Cmd{
				ToShellCmd("echo hello"),
			},
		},
		// no triggers, no files changed
		boilTestCase{
			steps: []Step{
				Step{
					Cmd: ToShellCmd("echo hello"),
				},
			},
			filesChanged: []string{},
			expected:     []Cmd{},
		},
		// one trigger, other files changed
		boilTestCase{
			steps: []Step{
				Step{
					Cmd:     ToShellCmd("echo hello"),
					Trigger: falseMatcher{},
				},
			},
			filesChanged: []string{"foo"},
			expected:     []Cmd{},
		},
		// one trigger, files changed, but it matches
		boilTestCase{
			steps: []Step{
				Step{
					Cmd:     ToShellCmd("echo hello"),
					Trigger: trueMatcher{},
				},
			},
			filesChanged: []string{"foo"},
			expected: []Cmd{
				ToShellCmd("echo hello"),
			},
		},
		// multiple triggers, multiple files changed
		boilTestCase{
			steps: []Step{
				Step{
					Cmd:     ToShellCmd("echo hello"),
					Trigger: trueMatcher{},
				},
				Step{
					Cmd:     ToShellCmd("echo world"),
					Trigger: falseMatcher{},
				},
			},
			filesChanged: []string{"foo", "bar"},
			expected: []Cmd{
				ToShellCmd("echo hello"),
			},
		},
	}

	for i, tc := range boilStepsTests {
		testBoilSteps(t, i, tc)
	}
}

func testBoilSteps(t *testing.T, i int, tc boilTestCase) {
	td := testutils.NewTempDirFixture(t)
	defer td.TearDown()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fc := make([]string, len(tc.filesChanged))
	for i, f := range tc.filesChanged {
		fc[i] = td.JoinPath(f)
	}
	defer os.Chdir(oldWD)
	err = os.Chdir(td.Path())
	if err != nil {
		t.Fatal(err)
	}
	actual := BoilSteps(tc.steps, fc)

	assert.ElementsMatchf(t, actual, tc.expected, "test case %d", i)
}
