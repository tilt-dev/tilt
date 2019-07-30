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
		runs := model.ToRuns(cmds)

		ref, err := f.b.DeprecatedFastBuildImage(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Sync{}, model.EmptyMatcher, runs, model.Cmd{})
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

		runs := model.ToRuns([]model.Cmd{model.ToShellCmd(oneCmd)})
		ref, err := f.b.DeprecatedFastBuildImage(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, nil, model.EmptyMatcher, runs, model.Cmd{})
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
