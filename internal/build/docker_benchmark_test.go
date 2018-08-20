//+build !skipcontainertests

package build

import (
	"fmt"
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

		digest, err := f.b.BuildDockerFromScratch(f.ctx, simpleDockerfile, []model.Mount{}, steps, model.Cmd{})
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
