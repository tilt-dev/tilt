package prompt

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/pkg/model"
)

const FakeURL = "http://localhost:10350/"

func TestOpenBrowser(t *testing.T) {
	f := newFixture()
	defer f.TearDown()

	f.prompt.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	assert.Contains(t, f.out.String(), "(space) to open the browser")

	f.input.nextRune <- ' '
	assert.Equal(t, FakeURL, f.b.WaitForURL(t))
}

func TestOpenStream(t *testing.T) {
	f := newFixture()
	defer f.TearDown()

	f.prompt.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	assert.Contains(t, f.out.String(), "(s) to stream logs")

	f.input.nextRune <- 's'

	action := f.st.WaitForAction(t, reflect.TypeOf(SwitchTerminalModeAction{}))
	assert.Equal(t, SwitchTerminalModeAction{Mode: store.TerminalModeStream}, action)
}

func TestOpenHUD(t *testing.T) {
	f := newFixture()
	defer f.TearDown()

	f.prompt.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	assert.Contains(t, f.out.String(), "(t) to open legacy terminal mode")

	f.input.nextRune <- 't'

	action := f.st.WaitForAction(t, reflect.TypeOf(SwitchTerminalModeAction{}))
	assert.Equal(t, SwitchTerminalModeAction{Mode: store.TerminalModeHUD}, action)
}

func TestInitOutput(t *testing.T) {
	f := newFixture()
	defer f.TearDown()

	f.prompt.SetInitOutput(bytes.NewBuffer([]byte("this is a warning\n")))
	f.prompt.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	assert.Contains(t, f.out.String(), `this is a warning

(space) to open the browser`)
}

type fixture struct {
	ctx    context.Context
	cancel func()
	out    *bufsync.ThreadSafeBuffer
	st     *store.TestingStore
	b      *fakeBrowser
	input  *fakeInput
	prompt *TerminalPrompt
}

func newFixture() *fixture {
	ctx, _, ta := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	out := bufsync.NewThreadSafeBuffer()
	st := store.NewTestingStore()
	st.WithState(func(state *store.EngineState) {
		state.TerminalMode = store.TerminalModePrompt
	})
	i := &fakeInput{ctx: ctx, nextRune: make(chan rune)}
	b := &fakeBrowser{url: make(chan string)}
	openInput := OpenInput(func() (TerminalInput, error) { return i, nil })

	url, _ := url.Parse(FakeURL)

	prompt := NewTerminalPrompt(ta, openInput, b.OpenURL, out, "localhost", model.WebURL(*url))
	return &fixture{
		ctx:    ctx,
		cancel: cancel,
		out:    out,
		st:     st,
		input:  i,
		b:      b,
		prompt: prompt,
	}
}

func (f *fixture) TearDown() {
	f.cancel()
}

type fakeInput struct {
	ctx      context.Context
	nextRune chan rune
}

func (i *fakeInput) Close() error { return nil }

func (i *fakeInput) ReadRune() (rune, error) {
	select {
	case r := <-i.nextRune:
		return r, nil
	case <-i.ctx.Done():
		return 0, i.ctx.Err()
	}
}

type fakeBrowser struct {
	url chan string
}

func (b *fakeBrowser) WaitForURL(t *testing.T) string {
	select {
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for url")
		return ""
	case url := <-b.url:
		return url
	}
}

func (b *fakeBrowser) OpenURL(url string, w io.Writer) error {
	b.url <- url
	return nil
}
