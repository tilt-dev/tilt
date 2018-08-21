package image

import (
	"context"
	"testing"
	"time"

	// needed by go-digest for FromString
	_ "crypto/sha256"

	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/wmclient/pkg/dirs"
	"github.com/windmilleng/wmclient/pkg/os/temp"
)

func TestCheckpointEmpty(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")
	d, c, ok := history.MostRecent(n1)
	if ok {
		t.Errorf("Expected no recent image found. Actual: %v, %v", d, c)
	}
}

func TestCheckpointOne(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")
	d1 := digest.FromString("digest1")
	c1 := history.CheckpointNow()
	history.Load(f.ctx, n1, d1, c1)

	d, c, ok := history.MostRecent(n1)
	if !ok || d != d1 || c != c1 {
		t.Errorf("Expected most recent image (%v, %v). Actual: (%v, %v)", c1, d1, c, d)
	}
}

func TestCheckpointAfter(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")

	d1 := digest.FromString("digest1")
	c1 := history.CheckpointNow()
	history.Load(f.ctx, n1, d1, c1)

	time.Sleep(time.Millisecond)

	d2 := digest.FromString("digest2")
	c2 := history.CheckpointNow()
	history.Load(f.ctx, n1, d2, c2)

	d, c, ok := history.MostRecent(n1)
	if !ok || d != d2 || c != c2 {
		t.Errorf("Expected most recent image (%v, %v). Actual: (%v, %v)", c2, d2, c, d)
	}
}

func TestCheckpointBefore(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")
	c0 := history.CheckpointNow()
	d0 := digest.FromString("digest0")
	time.Sleep(time.Millisecond)

	d1 := digest.FromString("digest1")
	c1 := history.CheckpointNow()
	history.Load(f.ctx, n1, d1, c1)
	history.Load(f.ctx, n1, d0, c0)

	d, c, ok := history.MostRecent(n1)
	if !ok || d != d1 || c != c1 {
		t.Errorf("Expected most recent image (%v, %v). Actual: (%v, %v)", c1, d1, c, d)
	}
}

func TestPersistence(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()
	history := f.history
	n1, _ := reference.ParseNormalizedNamed("image-name-1")

	d1 := digest.FromString("digest1")
	c1 := history.CheckpointNow()
	err := history.AddAndPersist(f.ctx, n1, d1, c1)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond)

	d2 := digest.FromString("digest2")
	c2 := history.CheckpointNow()
	err = history.AddAndPersist(f.ctx, n1, d2, c2)
	if err != nil {
		t.Fatal(err)
	}

	history2, err := NewImageHistory(f.ctx, f.dir)
	if err != nil {
		t.Fatal(err)
	}

	d, _, ok := history2.MostRecent(n1)
	if !ok || d != d2 {
		t.Errorf("Expected most recent image (%v). Actual: (%v)", d2, d)
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
