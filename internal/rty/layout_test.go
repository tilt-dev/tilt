package rty

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/windmilleng/tcell"
)

type LayoutTestCase struct {
	name   string
	width  int
	height int
	C      Component
}

func TestAll(t *testing.T) {
	f := newLayoutTestFixture(t)
	defer f.cleanUp()

	f.addMany(SimpleTextCases)
	f.addMany(StyledTextCases)
	f.addMany(BoxCases)
	f.addMany(StyleCases)

	for _, c := range f.cases {
		t.Run(c.name, func(t *testing.T) {
			actual := newTempCanvas(c.width, c.height, tcell.StyleDefault)
			g := &renderGlobals{prev: make(renderState), next: make(renderState)}
			r := renderFrame{
				canvas:  actual,
				globals: g,
			}
			defer func() {
				if e := recover(); e != nil {
					t.Fatalf("panic rendering: %v %s", e, debug.Stack())
				}
			}()
			r.RenderChild(c.C)
			if g.err != nil {
				t.Fatalf("error rendering: %v", g.err)
			}
			expected := f.loadGoldenFile(c.name)

			if !f.canvasesEqual(actual, expected) {
				err := f.displayAndMaybeWrite(c.name, actual, expected)
				t.Fatal(err)
			}
		})
	}
}

type fixture struct {
	t *testing.T

	cases []LayoutTestCase

	prefix string
	nextId int
}

func newLayoutTestFixture(t *testing.T) *fixture {
	return &fixture{
		t: t,
	}
}

func (f *fixture) cleanUp() {
}

func (f *fixture) add(width int, height int, c Component) {

	name := filepath.Join(f.prefix, fmt.Sprintf("%03d", f.nextId))
	f.nextId++
	f.cases = append(f.cases, LayoutTestCase{name, width, height, c})
}

func (f *fixture) addN(name string, width int, height int, c Component) {
	name = filepath.Join(f.prefix, name)
	f.cases = append(f.cases, LayoutTestCase{name, width, height, c})
}

func (f *fixture) addMany(addFunc func(f *fixture)) {
	funcName := runtime.FuncForPC(reflect.ValueOf(addFunc).Pointer()).Name()
	// this gives us something like "github.com/windmilleng/tilt/internal/rty.SimpleText"
	// we want SimpleText
	funcName = strings.Split(filepath.Base(funcName), ".")[1]
	f.push(funcName)
	defer f.pop()
	addFunc(f)
}

func (f *fixture) push(s string) {
	f.prefix = filepath.Join(f.prefix, s)
}

func (f *fixture) pop() {
	f.prefix = filepath.Dir(f.prefix)
}

func (f *fixture) canvasesEqual(actual, expected Canvas) bool {
	actualWidth, actualHeight := actual.Size()
	expectedWidth, expectedHeight := expected.Size()
	if actualWidth != expectedWidth || actualHeight != expectedHeight {
		return false
	}

	for x := 0; x < actualWidth; x++ {
		for y := 0; y < actualHeight; y++ {
			actualCh, _, actualStyle, _ := actual.GetContent(x, y)
			expectedCh, _, expectedStyle, _ := expected.GetContent(x, y)
			if actualCh != expectedCh || actualStyle != expectedStyle {
				return false
			}
		}
	}

	return true
}

var screen tcell.Screen

func (f *fixture) displayAndMaybeWrite(name string, actual, expected Canvas) error {
	if screen == nil {
		return nil
	}

	screen.Clear()
	actualWidth, actualHeight := actual.Size()
	expectedWidth, expectedHeight := expected.Size()

	printForTest(screen, 0, fmt.Sprintf("test %s", name))
	printForTest(screen, 1, "actual:")

	for x := 0; x < actualWidth; x++ {
		for y := 0; y < actualHeight; y++ {
			ch, _, style, _ := actual.GetContent(x, y)
			screen.SetContent(x, y+2, ch, nil, style)
		}
	}

	printForTest(screen, actualHeight+3, "expected:")

	for x := 0; x < expectedWidth; x++ {
		for y := 0; y < expectedHeight; y++ {
			ch, _, style, _ := expected.GetContent(x, y)
			screen.SetContent(x, y+actualHeight+4, ch, nil, style)
		}
	}

	screen.Show()

	for {
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Rune() {
			case 'y':
				return f.writeGoldenFile(name, actual)
			case 'n':
				return nil
			}
		}
	}
}

func printForTest(screen tcell.Screen, y int, text string) {
	for x, ch := range text {
		screen.SetContent(x, y, ch, nil, tcell.StyleDefault)
	}
}

type caseData struct {
	Width  int
	Height int
	Cells  []caseCell
}

type caseCell struct {
	Ch    rune
	Style tcell.Style
}

func (f *fixture) filename(name string) string {
	return filepath.Join("testdata", strings.Replace(name, "/", "_", -1)+".gob")
}

func (f *fixture) loadGoldenFile(name string) Canvas {
	fi, err := os.Open(f.filename(name))
	if err != nil {
		return newTempCanvas(1, 1, tcell.StyleDefault)
	}
	defer fi.Close()

	dec := gob.NewDecoder(fi)
	var d caseData
	err = dec.Decode(&d)
	if err != nil {
		return newTempCanvas(1, 1, tcell.StyleDefault)
	}

	c := newTempCanvas(d.Width, d.Height, tcell.StyleDefault)
	for i, cell := range d.Cells {
		c.SetContent(i%d.Width, i/d.Width, cell.Ch, nil, cell.Style)
	}

	return c
}

func (f *fixture) writeGoldenFile(name string, actual Canvas) error {
	fi, err := os.Create(f.filename(name))
	if err != nil {
		return err
	}

	width, height := actual.Size()
	d := caseData{
		Width:  width,
		Height: height,
	}

	// iterative over y first so we write by rows
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			ch, _, style, _ := actual.GetContent(x, y)
			d.Cells = append(d.Cells, caseCell{Ch: ch, Style: style})
		}
	}

	enc := gob.NewEncoder(fi)
	return enc.Encode(d)
}

func TestMain(m *testing.M) {
	if s := os.Getenv("RTY_INTERACTIVE"); s != "" {
		var err error
		screen, err = tcell.NewTerminfoScreen()
		if err != nil {
			log.Fatal(err)
		}
		screen.Init()
		defer screen.Fini()
	}
	r := m.Run()
	if screen != nil {
		screen.Fini()
	}
	if r != 0 && screen == nil {
		log.Printf("To update golden files, run with env variable RTY_INTERACTIVE=1 and hit y/n on each case to overwrite (or not)")
	}
	os.Exit(r)
}
