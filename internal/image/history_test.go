package image

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	// needed by go-digest for FromString
	_ "crypto/sha256"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/wmclient/pkg/dirs"
	"github.com/windmilleng/wmclient/pkg/os/temp"
)

const basicDockerfile = build.Dockerfile("FROM alpine")

func TestCheckpointEmpty(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")
	service := model.Service{
		DockerfileText: "FROM alpine",
	}
	d, c, ok := history.MostRecent(n1, service)
	if ok {
		t.Errorf("Expected no recent image found. Actual: %v, %v", d, c)
	}
}

func TestCheckpointOne(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")
	c1 := history.CheckpointNow()
	service := model.Service{
		DockerfileText: "FROM alpine",
	}
	_, _, err := history.addInMemory(f.ctx, n1, c1, service)
	if err != nil {
		f.t.Fatal(err)
	}

	n, c, ok := history.MostRecent(n1, service)
	if !ok || n != n1 || c != c1 {
		t.Errorf("Expected most recent image (%v, %v). Actual: (%v, %v)", c1, n1, c, n)
	}
}

func TestCheckpointAfter(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")

	c1 := history.CheckpointNow()
	service := model.Service{
		DockerfileText: "FROM alpine",
	}
	_, _, err := history.addInMemory(f.ctx, n1, c1, service)
	if err != nil {
		f.t.Fatal(err)
	}

	time.Sleep(time.Millisecond)

	c2 := history.CheckpointNow()
	history.addInMemory(f.ctx, n1, c2, service)

	n, c, ok := history.MostRecent(n1, service)
	if !ok || c != c2 {
		t.Errorf("Expected most recent image (%v, %v). Actual: (%v, %v)", c2, n1, c, n)
	}
}

func TestCheckpointBefore(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")
	c0 := history.CheckpointNow()
	time.Sleep(time.Millisecond)

	c1 := history.CheckpointNow()
	service := model.Service{
		DockerfileText: "FROM alpine",
	}
	_, _, err := history.addInMemory(f.ctx, n1, c1, service)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = history.addInMemory(f.ctx, n1, c0, service)
	if err != nil {
		t.Fatal(err)
	}

	n, c, ok := history.MostRecent(n1, service)
	if !ok || n != n1 || c != c1 {
		t.Errorf("Expected most recent image (%v, %v). Actual: (%v, %v)", c1, n1, c, n)
	}
}

func TestPersistence(t *testing.T) {
	f := newFixture(t)
	fmt.Printf("temppath: %s\n", f.temp.Path())
	//defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")

	c1 := history.CheckpointNow()
	service := model.Service{
		DockerfileText: "FROM alpine",
	}
	err := history.AddAndPersist(f.ctx, n1, c1, service)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond)

	c2 := history.CheckpointNow()
	err = history.AddAndPersist(f.ctx, n1, c2, service)
	if err != nil {
		t.Fatal(err)
	}
	oldLen := f.getLengthOfFile()

	history2, err := NewImageHistory(f.ctx, f.dir)
	if err != nil {
		t.Fatal(err)
	}

	newLen := f.getLengthOfFile()

	n, _, ok := history2.MostRecent(n1, service)
	if !ok || n1 != n {
		t.Errorf("Expected most recent image (%v). Actual: (%v)", n1, n)
	}

	if oldLen != newLen {
		t.Errorf("Expected the length of the history file to not change when reloaded. Old length was %d, new length was %d", oldLen, newLen)
	}
}

func TestCheckpointDoesntMatchHash(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")

	c1 := history.CheckpointNow()
	service := model.Service{
		DockerfileText: "FROM alpine",
	}
	history.addInMemory(f.ctx, n1, c1, service)

	time.Sleep(time.Millisecond)

	c2 := history.CheckpointNow()
	history.addInMemory(f.ctx, n1, c2, service)

	service2 := model.Service{
		DockerfileText: "FROM alpine",
		Entrypoint:     model.Cmd{Argv: []string{"echo", "hi"}},
	}
	d, c, ok := history.MostRecent(n1, service2)
	if ok {
		t.Errorf("Expected no image, got digest: %+v, checkpoint: %+v", d, c)
	}
}

type fixture struct {
	t       *testing.T
	ctx     context.Context
	temp    *temp.TempDir
	dir     *dirs.WindmillDir
	history ImageHistory
}

func newFixture(t *testing.T) *fixture {
	temp, err := temp.NewDir(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	ctx := testutils.CtxForTest()
	dir := dirs.NewWindmillDirAt(temp.Path())
	history, err := NewImageHistory(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	return &fixture{
		t:       t,
		ctx:     ctx,
		temp:    temp,
		dir:     dir,
		history: history,
	}
}

func (f *fixture) tearDown() {
	err := f.temp.TearDown()
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) getLengthOfFile() int {
	h, err := f.dir.OpenFile(filepath.Join("tilt", "images.json"), os.O_RDONLY, 0755)
	if err != nil {
		f.t.Fatal(err)
	}
	c, err := lineCounter(h)
	if err != nil {
		f.t.Fatal(err)
	}

	return c
}

func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}
