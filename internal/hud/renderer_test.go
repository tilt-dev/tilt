package hud

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/hud/view"

	"github.com/windmilleng/tcell"
)

func TestRender(t *testing.T) {
	tf := newRendererTestFixture()

	v := view.View{
		Resources: []view.Resource{
			{
				Name:             "foo",
				DirectoryWatched: "bar",
			},
		},
	}

	tf.renderer.Render(v)

	expectedContent := []string{"", "foo — not yet deployed", "  Watching bar/", "  BUILD: no build yet"}

	assert.Equal(t, expectedContent, tf.fakeScreen.Lines())
}

type rendererTestFixture struct {
	renderer   *Renderer
	fakeScreen *fakeScreen
}

func newRendererTestFixture() *rendererTestFixture {
	fs := fakeScreen{}
	renderer := Renderer{
		screen:   &fs,
		screenMu: new(sync.Mutex),
	}
	return &rendererTestFixture{
		renderer:   &renderer,
		fakeScreen: &fs,
	}
}

type fakeScreen struct {
	t       *testing.T
	content [][]rune
}

var _ tcell.Screen = &fakeScreen{}

func (fs *fakeScreen) Lines() []string {
	var ret []string
	for _, row := range fs.content {
		ret = append(ret, string(row))
	}
	return ret
}

func (fs *fakeScreen) Init() error {
	fs.t.Fatal("Init not implemented in fake screen")
	return nil
}

func (fs *fakeScreen) Fini() {
	fs.t.Fatal("Fini not implemented in fake screen")
}

func (fs *fakeScreen) Clear() {
}

func (fs *fakeScreen) Fill(rune, tcell.Style) {
	fs.t.Fatal("Fill not implemented in fake screen")
}

func (fs *fakeScreen) SetCell(x int, y int, style tcell.Style, ch ...rune) {
	fs.t.Fatal("SetCell not implemented in fake screen")
}

func (fs *fakeScreen) GetContent(x, y int) (mainc rune, combc []rune, style tcell.Style, width int) {
	fs.t.Fatal("GetContent not implemented in fake screen")
	return 'a', []rune{}, 0, 0
}

func (fs *fakeScreen) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if len(fs.content) <= y {
		oldContent := fs.content
		fs.content = make([][]rune, y+1)
		copy(fs.content, oldContent)
	}

	if len(fs.content[y]) <= x {
		oldRow := fs.content[y]
		fs.content[y] = make([]rune, x+1)
		copy(fs.content[y], oldRow)
	}

	fs.content[y][x] = mainc
}

func (fs *fakeScreen) SetStyle(style tcell.Style) {
	fs.t.Fatal("SetStyle not implemented in fake screen")
}

func (fs *fakeScreen) ShowCursor(x int, y int) {
	fs.t.Fatal("ShowCursor not implemented in fake screen")
}

func (fs *fakeScreen) HideCursor() {
	fs.t.Fatal("HideCursor not implemented in fake screen")
}

func (fs *fakeScreen) Size() (int, int) {
	fs.t.Fatal("Size not implemented in fake screen")
	return 0, 0
}

func (fs *fakeScreen) PollEvent() tcell.Event {
	fs.t.Fatal("PollEvent not implemented in fake screen")
	return nil
}

func (fs *fakeScreen) PostEvent(ev tcell.Event) error {
	fs.t.Fatal("PostEvent not implemented in fake screen")
	return nil
}

func (fs *fakeScreen) PostEventWait(ev tcell.Event) {
	fs.t.Fatal("PostEventWait not implemented in fake screen")
}

func (fs *fakeScreen) EnableMouse() {
	fs.t.Fatal("EnableMouse not implemented in fake screen")
}

func (fs *fakeScreen) DisableMouse() {
	fs.t.Fatal("DisableMouse not implemented in fake screen")
}

func (fs *fakeScreen) HasMouse() bool {
	fs.t.Fatal("HasMouse not implemented in fake screen")
	return false
}

func (fs *fakeScreen) Colors() int {
	fs.t.Fatal("Colors not implemented in fake screen")
	return 0
}

func (fs *fakeScreen) Show() {
}

func (fs *fakeScreen) Sync() {
	fs.t.Fatal("Sync not implemented in fake screen")
}

func (fs *fakeScreen) CharacterSet() string {
	fs.t.Fatal("CharacterSet not implemented in fake screen")
	return ""
}

func (fs *fakeScreen) RegisterRuneFallback(r rune, subst string) {
	fs.t.Fatal("RegisterRuneFallback not implemented in fake screen")
}

func (fs *fakeScreen) UnregisterRuneFallback(r rune) {
	fs.t.Fatal("UnregisterRuneFallback not implemented in fake screen")
}

func (fs *fakeScreen) CanDisplay(r rune, checkFallbacks bool) bool {
	fs.t.Fatal("CanDisplay not implemented in fake screen")
	return false
}

func (fs *fakeScreen) Resize(int, int, int, int) {
	fs.t.Fatal("Resize not implemented in fake screen")
}

func (fs *fakeScreen) HasKey(tcell.Key) bool {
	fs.t.Fatal("HasKey not implemented in fake screen")
	return false
}
