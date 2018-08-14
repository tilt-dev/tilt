package image

import (
	"testing"
	"time"

	// needed by go-digest for FromString
	_ "crypto/sha256"

	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
)

func TestCheckpointEmpty(t *testing.T) {
	history := NewImageHistory()
	n1, _ := reference.ParseNormalizedNamed("image-name-1")
	d, c, ok := history.MostRecent(n1)
	if ok {
		t.Errorf("Expected no recent image found. Actual: %v, %v", d, c)
	}
}

func TestCheckpointOne(t *testing.T) {
	history := NewImageHistory()
	n1, _ := reference.ParseNormalizedNamed("image-name-1")
	d1 := digest.FromString("digest1")
	c1 := history.CheckpointNow()
	history.Add(n1, d1, c1)
	d, c, ok := history.MostRecent(n1)
	if !ok || d != d1 || c != c1 {
		t.Errorf("Expected most recent image (%v, %v). Actual: (%v, %v)", c1, d1, c, d)
	}
}

func TestCheckpointAfter(t *testing.T) {
	history := NewImageHistory()
	n1, _ := reference.ParseNormalizedNamed("image-name-1")

	d1 := digest.FromString("digest1")
	c1 := history.CheckpointNow()
	history.Add(n1, d1, c1)

	time.Sleep(time.Millisecond)

	d2 := digest.FromString("digest2")
	c2 := history.CheckpointNow()
	history.Add(n1, d2, c2)

	d, c, ok := history.MostRecent(n1)
	if !ok || d != d2 || c != c2 {
		t.Errorf("Expected most recent image (%v, %v). Actual: (%v, %v)", c2, d2, c, d)
	}
}

func TestCheckpointBefore(t *testing.T) {
	history := NewImageHistory()
	n1, _ := reference.ParseNormalizedNamed("image-name-1")
	c0 := history.CheckpointNow()
	d0 := digest.FromString("digest0")
	time.Sleep(time.Millisecond)

	d1 := digest.FromString("digest1")
	c1 := history.CheckpointNow()
	history.Add(n1, d1, c1)
	history.Add(n1, d0, c0)

	d, c, ok := history.MostRecent(n1)
	if !ok || d != d1 || c != c1 {
		t.Errorf("Expected most recent image (%v, %v). Actual: (%v, %v)", c1, d1, c, d)
	}
}
