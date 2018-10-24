package rty

import (
	"strconv"
	"strings"
	"testing"
)

func TestElementScroll(t *testing.T) {
	f := newElementScrollTestFixture(t)
	defer f.cleanUp()

	f.run("initial scroll state")

	f.down()
	f.run("scrolled down one")

	for i := 0; i < 9; i++ {
		f.down()
	}
	f.run("scrolled down ten")

	for i := 0; i < 20; i++ {
		f.down()
	}
	f.run("scrolled all the way down")

	for i := 0; i < 3; i++ {
		f.up()
	}
	f.run("scrolled 3 up from bottom")

	for i := 0; i < 100; i++ {
		f.up()
	}
	f.run("scrolled all the way back up")
}

type elementScrollTestFixture struct {
	fixture *fixture
}

func newElementScrollTestFixture(t *testing.T) *elementScrollTestFixture {
	return &elementScrollTestFixture{
		fixture: newLayoutTestFixture(t),
	}
}

func (f *elementScrollTestFixture) layout() Component {
	var childrenNames []string
	for i := 0; i < 20; i++ {
		childrenNames = append(childrenNames, strconv.FormatInt(int64(i+1), 10))
	}

	l, selectedName := f.fixture.r.RegisterElementScroll("items", childrenNames)

	var components []Component
	for _, n := range childrenNames {
		c := NewLines()
		if selectedName == n {
			c.Add(TextString("SELECTED---->"))
		}
		for j := 0; j < 3; j++ {
			c.Add(TextString(strings.Repeat(n, 10)))
		}
		components = append(components, c)
	}

	for _, c := range components {
		l.Add(c)
	}

	return l
}

func (f *elementScrollTestFixture) run(name string) {
	f.fixture.run(name, 20, 10, f.layout())
}

func (f *elementScrollTestFixture) down() {
	f.fixture.render(20, 10, f.layout())
	f.fixture.r.ElementScroller("items").DownElement()
}

func (f *elementScrollTestFixture) up() {
	f.fixture.render(20, 10, f.layout())
	f.fixture.r.ElementScroller("items").UpElement()
}

func (f *elementScrollTestFixture) cleanUp() {
	f.fixture.cleanUp()
}
