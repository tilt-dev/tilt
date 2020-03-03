package rty

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/gdamore/tcell"
)

const testDataDir = "testdata"

// Whitelist characters allowed in a name, because they will be used to create
// filenames.
//
// Forbid filenames with colons because they mess up the Windows git client :(
var validNameRegexp = regexp.MustCompile("^[a-zA-Z0-9 .,_-]+$")

type InteractiveTester struct {
	usedNames         map[string]bool
	dummyScreen       tcell.SimulationScreen
	interactiveScreen tcell.Screen
	rty               RTY
	t                 ErrorReporter
}

type ErrorReporter interface {
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

func NewInteractiveTester(t ErrorReporter, screen tcell.Screen) InteractiveTester {
	dummyScreen := tcell.NewSimulationScreen("")
	err := dummyScreen.Init()
	assert.NoError(t, err)

	return InteractiveTester{
		usedNames:         make(map[string]bool),
		dummyScreen:       dummyScreen,
		interactiveScreen: screen,
		rty:               NewRTY(dummyScreen, t),
		t:                 t,
	}
}

func (i *InteractiveTester) T() ErrorReporter {
	return i.t
}

func (i *InteractiveTester) Run(name string, width int, height int, c Component) {
	err := i.runCaptureError(name, width, height, c)
	if err != nil {
		i.t.Errorf("error rendering %s: %v", name, err)
	}
	i.dummyScreen.Clear()
}

func (i *InteractiveTester) render(width int, height int, c Component) Canvas {
	actual := newScreenCanvas(i.dummyScreen, i.t)
	i.dummyScreen.SetSize(width, height)
	defer func() {
		if e := recover(); e != nil {
			i.t.Fatalf("panic rendering: %v %s", e, debug.Stack())
		}
	}()
	i.rty.Render(c)
	return actual
}

// Returns an error if rendering failed.
// If any other failure is encountered, fails via `i.t`'s `testing.T` and returns `nil`.
func (i *InteractiveTester) runCaptureError(name string, width int, height int, c Component) error {
	_, ok := i.usedNames[name]
	if ok {
		i.t.Fatalf("test name '%s' was already used", name)
	}

	if !validNameRegexp.MatchString(name) {
		i.t.Fatalf("test name has invalid characters: %s", name)
	}

	actual := i.render(width, height, c)
	expected := i.loadGoldenFile(name)

	eq := canvasesEqual(actual, expected)
	if !eq {
		updated, err := i.displayAndMaybeWrite(name, actual, expected)
		if err == nil {
			if !updated {
				err = errors.New("actual rendering didn't match expected")
			}
		}
		if err != nil {
			i.t.Errorf("%s: %v", name, err)
		}
	}
	return nil
}

func canvasesEqual(actual, expected Canvas) bool {
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

func (i *InteractiveTester) renderDiff(screen tcell.Screen, name string, actual, expected Canvas, highlightDiff bool) error {
	screen.Clear()

	actualWidth, actualHeight := actual.Size()
	expectedWidth, expectedHeight := expected.Size()

	curHeight := 0

	printForTest(screen, curHeight, "y: accept, n: reject, d: diff, q: quit")
	curHeight++

	printForTest(screen, curHeight, fmt.Sprintf("test: %s", name))
	curHeight++

	printForTest(screen, curHeight, "actual:")
	curHeight++

	for y := 0; y < actualHeight; y++ {
		for x := 0; x < actualWidth; x++ {
			ch, _, style, _ := actual.GetContent(x, y)
			if highlightDiff {
				expectedCh, _, expectedStyle, _ := expected.GetContent(x, y)
				if ch != expectedCh || style != expectedStyle {
					style = style.Reverse(true)
				}
			}

			screen.SetContent(x, curHeight, ch, nil, style)
		}
		curHeight++
	}

	curHeight++

	printForTest(screen, curHeight, "expected:")

	curHeight++

	for y := 0; y < expectedHeight; y++ {
		for x := 0; x < expectedWidth; x++ {
			ch, _, style, _ := expected.GetContent(x, y)
			if highlightDiff {
				actualCh, _, actualStyle, _ := actual.GetContent(x, y)
				if ch != actualCh || style != actualStyle {
					style = style.Reverse(true)
				}
			}

			screen.SetContent(x, curHeight, ch, nil, style)
		}
		curHeight++
	}

	screen.Show()

	return nil
}

func (i *InteractiveTester) displayAndMaybeWrite(name string, actual, expected Canvas) (updated bool, err error) {
	screen := i.interactiveScreen
	if screen == nil {
		return false, nil
	}

	highlightDiff := false

	for {
		err := i.renderDiff(screen, name, actual, expected, highlightDiff)
		if err != nil {
			return false, err
		}

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Rune() {
			case 'y':
				return true, i.writeGoldenFile(name, actual)
			case 'n':
				return false, errors.New("user indicated expected output was not as desired")
			case 'd':
				highlightDiff = !highlightDiff
			case 'q':
				fmt.Println("User exited by pressing 'q'")
				screen.Fini()
				os.Exit(1)
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

func (i *InteractiveTester) filename(name string) string {
	return filepath.Join(testDataDir, strings.Replace(name, "/", "_", -1)+".gob")
}

func (i *InteractiveTester) loadGoldenFile(name string) Canvas {
	fi, err := os.Open(i.filename(name))
	if err != nil {
		return newTempCanvas(1, 1, tcell.StyleDefault, i.t)
	}
	defer func() {
		err := fi.Close()
		if err != nil {
			log.Printf("error closing file %s\n", fi.Name())
		}
	}()

	dec := gob.NewDecoder(fi)
	var d caseData
	err = dec.Decode(&d)
	if err != nil {
		return newTempCanvas(1, 1, tcell.StyleDefault, i.t)
	}

	c := newTempCanvas(d.Width, d.Height, tcell.StyleDefault, i.t)
	for i, cell := range d.Cells {
		x := i % d.Width
		y := i / d.Width
		c.SetContent(x, y, cell.Ch, nil, cell.Style)
	}

	return c
}

func (i *InteractiveTester) writeGoldenFile(name string, actual Canvas) error {
	_, err := os.Stat(testDataDir)
	if os.IsNotExist(err) {
		err := os.Mkdir(testDataDir, os.FileMode(0755))
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	fi, err := os.Create(i.filename(name))
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

// unfortunately, tcell misbehaves if we try to make a new Screen for every test
// this function is intended for use from a `TestMain`, so that we can have a global Screen across all tests in the package
func InitScreenAndRun(m *testing.M, screen *tcell.Screen) {
	if s := os.Getenv("RTY_INTERACTIVE"); s != "" {
		var err error
		*screen, err = tcell.NewTerminfoScreen()
		if err != nil {
			log.Fatal(err)
		}
		err = (*screen).Init()
		if err != nil {
			log.Fatal(err)
		}
	}

	r := m.Run()
	if *screen != nil {
		(*screen).Fini()
	}

	if r != 0 && *screen == nil {
		log.Printf("To update golden files, run with env variable RTY_INTERACTIVE=1 and hit y/n on each case to overwrite (or not)")
	}
	os.Exit(r)
}
