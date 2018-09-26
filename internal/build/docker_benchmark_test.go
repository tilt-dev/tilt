//+build !skipcontainertests

package build

import (
	"fmt"
	"strings"
	"testing"

	"github.com/windmilleng/tilt/internal/model"
)

func BenchmarkBuildTenSteps(b *testing.B) {
	run := func() {
		f := newDockerBuildFixture(b)
		defer f.teardown()

		cmds := []model.Cmd{}
		for i := 0; i < 10; i++ {
			cmds = append(cmds, model.ToShellCmd(fmt.Sprintf("echo -n %d > hi", i)))
		}
		steps := model.ToSteps(cmds)

		ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, model.EmptyMatcher, steps, model.Cmd{})
		if err != nil {
			b.Fatal(err)
		}

		expected := []expectedFile{
			expectedFile{Path: "hi", Contents: "9"},
		}
		f.assertFilesInImage(ref, expected)
	}
	for i := 0; i < b.N; i++ {
		run()
	}
}

func BenchmarkBuildTenStepsInOne(b *testing.B) {
	run := func() {
		f := newDockerBuildFixture(b)
		defer f.teardown()

		allCmds := make([]string, 10)
		for i := 0; i < 10; i++ {
			allCmds[i] = fmt.Sprintf("echo -n %d > hi", i)
		}

		oneCmd := strings.Join(allCmds, " && ")

		steps := model.ToSteps([]model.Cmd{model.ToShellCmd(oneCmd)})
		ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, nil, model.EmptyMatcher, steps, model.Cmd{})
		if err != nil {
			b.Fatal(err)
		}

		expected := []expectedFile{
			expectedFile{Path: "hi", Contents: "9"},
		}
		f.assertFilesInImage(ref, expected)
	}
	for i := 0; i < b.N; i++ {
		run()
	}
}

func BenchmarkIterativeBuildTenTimes(b *testing.B) {
	f := newDockerBuildFixture(b)
	defer f.teardown()
	steps := model.ToSteps([]model.Cmd{model.ToShellCmd("echo 1 >> hi")})
	ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, nil, model.EmptyMatcher, steps, model.Cmd{})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			ref, err = f.b.BuildImageFromExisting(f.ctx, ref, nil, model.EmptyMatcher, steps)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
