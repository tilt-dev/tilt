//+build !skipcontainertests

package build

import (
	"testing"

	"github.com/windmilleng/tilt/internal/model"
)

func BenchmarkExecInContainer(b *testing.B) {
	f := newDockerBuildFixture(b)
	defer f.teardown()

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		b.Fatal(err)
	}

	cmd := model.ToShellCmd("echo hello") // expect to take < 10ms

	// start a container that does nothing but hangs around so we can run stuff on it
	cID := f.startContainer(f.ctx, containerConfigRunCmd(ref, model.Cmd{Argv: []string{"sleep", "300"}}))

	run := func() {
		err := f.dcli.ExecInContainer(f.ctx, cID, cmd)
		if err != nil {
			f.t.Fatal(err)
		}
	}

	for i := 0; i < b.N; i++ {
		run()
	}
}
