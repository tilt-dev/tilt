//+build !skipcontainertests

package build

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/windmilleng/tilt/internal/model"
)

func BenchmarkBuildTenSteps(b *testing.B) {
	run := func() {
		f := newDockerBuildFixture(b)
		defer f.teardown()

		steps := []model.Cmd{}
		for i := 0; i < 10; i++ {
			steps = append(steps, model.ToShellCmd(fmt.Sprintf("echo -n %d > hi", i)))
		}

		digest, err := f.b.BuildDockerFromScratch(f.ctx, simpleDockerfile, []model.Mount{}, steps, model.Cmd{}, os.Stdout)
		if err != nil {
			b.Fatal(err)
		}

		expected := []expectedFile{
			expectedFile{path: "hi", contents: "9"},
		}
		f.assertFilesInImage(string(digest), expected)
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

		steps := []model.Cmd{model.ToShellCmd(oneCmd)}
		digest, err := f.b.BuildDockerFromScratch(f.ctx, simpleDockerfile, []model.Mount{}, steps, model.Cmd{}, os.Stdout)
		if err != nil {
			b.Fatal(err)
		}

		expected := []expectedFile{
			expectedFile{path: "hi", contents: "9"},
		}
		f.assertFilesInImage(string(digest), expected)
	}
	for i := 0; i < b.N; i++ {
		run()
	}
}

func BenchmarkIterativeBuildTenTimes(b *testing.B) {
	f := newDockerBuildFixture(b)
	defer f.teardown()
	steps := []model.Cmd{model.ToShellCmd("echo 1 >> hi")}
	digest, err := f.b.BuildDockerFromScratch(f.ctx, simpleDockerfile, []model.Mount{}, steps, model.Cmd{}, os.Stdout)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			digest, err = f.b.BuildDockerFromExisting(f.ctx, digest, nil, steps, os.Stdout)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
