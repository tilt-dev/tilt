//+build !skipcontainertests

// NOTE(maia): this benchmark lives in `build` instead of `docker` for access to
// dockerBuildFixture, which is too gnarly to unwire from the build pkg right now.
package build

import (
	"io/ioutil"
	"testing"

	"github.com/windmilleng/tilt/internal/model"
)

func BenchmarkExecInContainer(b *testing.B) {
	f := newDockerBuildFixture(b)
	defer f.teardown()

	ref, err := f.b.DeprecatedFastBuildImage(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, nil, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		b.Fatal(err)
	}

	cmd := model.ToShellCmd("echo hello") // expect to take < 10ms

	// start a container that does nothing but hangs around so we can run stuff on it
	cID := f.startContainer(f.ctx, containerConfigRunCmd(ref, model.Cmd{Argv: []string{"sleep", "300"}}))

	run := func() {
		err := f.dCli.ExecInContainer(f.ctx, cID, cmd, ioutil.Discard)
		if err != nil {
			f.t.Fatal(err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		run()
	}
}
