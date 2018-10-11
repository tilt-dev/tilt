package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.PodLogs = "hello world!"
	name := model.ManifestName("server")
	ref := k8s.MustParseNamedTagged("re.po/project/myapp:tilt-936a185caaa266bb")
	prevBuild := store.BuildResult{}
	build := store.BuildResult{Image: ref}

	f.plm.PostProcessBuild(f.ctx, name, build, prevBuild)
	err := f.out.WaitUntilContains("hello world!", time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLogActions(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.PodLogs = "hello world!\ngoodbye world!\n"
	name := model.ManifestName("server")
	ref := k8s.MustParseNamedTagged("re.po/project/myapp:tilt-936a185caaa266bb")
	prevBuild := store.BuildResult{}
	build := store.BuildResult{Image: ref}

	f.plm.PostProcessBuild(f.ctx, name, build, prevBuild)
	f.ConsumeLogActionsUntil("hello world!")
}

func TestLogsFailed(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.PodLogs = "hello world!"
	f.kClient.PodsWithImageError = fmt.Errorf("my-error")
	name := model.ManifestName("server")
	ref := k8s.MustParseNamedTagged("re.po/project/myapp:tilt-936a185caaa266bb")
	prevBuild := store.BuildResult{}
	build := store.BuildResult{Image: ref}

	f.plm.PostProcessBuild(f.ctx, name, build, prevBuild)
	err := f.out.WaitUntilContains("Error streaming server logs", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, f.out.String(), "my-error")
}

type plmFixture struct {
	*tempdir.TempDirFixture
	ctx     context.Context
	kClient *k8s.FakeK8sClient
	plm     *PodLogManager
	cancel  func()
	out     *bufsync.ThreadSafeBuffer
	store   *store.Store
}

func newPLMFixture(t *testing.T) *plmFixture {
	f := tempdir.NewTempDirFixture(t)
	kClient := k8s.NewFakeK8sClient()
	st := store.NewStore()
	dd := NewDeployDiscovery(kClient, st)
	plm := NewPodLogManager(kClient, dd, st)

	out := bufsync.NewThreadSafeBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = logger.WithLogger(ctx, l)
	ctx = output.WithOutputter(ctx, output.NewOutputter(l))

	return &plmFixture{
		TempDirFixture: f,
		kClient:        kClient,
		plm:            plm,
		ctx:            ctx,
		cancel:         cancel,
		out:            out,
		store:          st,
	}
}

func (f *plmFixture) ConsumeLogActionsUntil(expected string) {
	out := []byte{}
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			f.T().Fatalf("Timeout. Collected output: %s", string(out))
		case action := <-f.store.Actions():
			podLog, ok := action.(PodLogAction)
			if !ok {
				f.T().Errorf("Expected action type PodLogAction. Actual: %T", action)
			}
			out = append(out, podLog.Log...)
			if !strings.Contains(string(out), expected) {
				continue
			}

			// we're done!
			return
		}
	}
}

func (f *plmFixture) TearDown() {
	f.cancel()
	f.TempDirFixture.TearDown()
}
