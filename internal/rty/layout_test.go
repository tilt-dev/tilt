package rty

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pkg/errors"

	"github.com/windmilleng/tcell"
)

var usedNames = make(map[string]bool)

const testDataDir = "testdata"

type fixture struct {
	t  *testing.T
	r  RTY
	sc tcell.SimulationScreen
}

func newLayoutTestFixture(t *testing.T) *fixture {
	sc := tcell.NewSimulationScreen("")
	err := sc.Init()
	assert.NoError(t, err)
	return &fixture{
		t:  t,
		r:  NewRTY(sc),
		sc: sc,
	}
}

func (f *fixture) cleanUp() {
}

func (f *fixture) run(name string, width int, height int, c Component) {
	err := f.runCaptureError(name, width, height, c)
	if err != nil {
		f.t.Errorf("error rendering %s: %v", name, err)
	}
}

func (f *fixture) render(width int, height int, c Component) (Canvas, error) {
	actual := newScreenCanvas(f.sc)
	f.sc.SetSize(width, height)
	defer func() {
		if e := recover(); e != nil {
			f.t.Errorf("panic rendering: %v %s", e, debug.Stack())
		}
	}()
	err := f.r.Render(c)
	return actual, err
}

// Returns an error if rendering failed.
// If any other failure is encountered, fails via `f.t`'s `testing.T` and returns `nil`.
func (f *fixture) runCaptureError(name string, width int, height int, c Component) error {
	_, ok := usedNames[name]
	if ok {
		f.t.Fatalf("test name '%s' was already used", name)
	}

	actual, err := f.render(width, height, c)
	if err != nil {
		return errors.Wrapf(err, "error rendering %s", name)
	}

	expected := f.loadGoldenFile(name)

	if !f.canvasesEqual(actual, expected) {
		updated, err := f.displayAndMaybeWrite(name, actual, expected)
		if err == nil {
			if !updated {
				err = errors.New("actual rendering didn't match expected")
			}
		}
		if err != nil {
			f.t.Errorf("%s: %v", name, err)
		}
	}
	return nil
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

func (f *fixture) displayAndMaybeWrite(name string, actual, expected Canvas) (updated bool, err error) {
	if screen == nil {
		return false, nil
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
				return true, f.writeGoldenFile(name, actual)
			case 'n':
				return false, errors.New("user indicated expected output was not as desired")
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
	return filepath.Join(testDataDir, strings.Replace(name, "/", "_", -1)+".gob")
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
	_, err := os.Stat(testDataDir)
	if os.IsNotExist(err) {
		err := os.Mkdir(testDataDir, os.FileMode(0755))
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
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
