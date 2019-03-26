//+build !skipcontainertests

package build

import (
	"fmt"
	"strings"
	"testing"

	"github.com/windmilleng/tilt/internal/model"
)

func BenchmarkBuildTenRuns(b *testing.B) {
	run := func() {
		f := newDockerBuildFixture(b)
		defer f.teardown()

		cmds := []model.Cmd{}
		for i := 0; i < 10; i++ {
			cmds = append(cmds, model.ToShellCmd(fmt.Sprintf("echo -n %d > hi", i)))
		}
		runs := model.ToRuns(f.Path(), cmds)

		ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, model.EmptyMatcher, runs, model.Cmd{})
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

func BenchmarkBuildTenRunsInOne(b *testing.B) {
	run := func() {
		f := newDockerBuildFixture(b)
		defer f.teardown()

		allCmds := make([]string, 10)
		for i := 0; i < 10; i++ {
			allCmds[i] = fmt.Sprintf("echo -n %d > hi", i)
		}

		oneCmd := strings.Join(allCmds, " && ")

		runs := model.ToRuns(f.Path(), []model.Cmd{model.ToShellCmd(oneCmd)})
		ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, nil, model.EmptyMatcher, runs, model.Cmd{})
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
	runs := model.ToRuns(f.Path(), []model.Cmd{model.ToShellCmd("echo 1 >> hi")})
	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, nil, model.EmptyMatcher, runs, model.Cmd{})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			ref, err = f.b.BuildImageFromExisting(f.ctx, f.ps, ref, nil, model.EmptyMatcher, runs)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
