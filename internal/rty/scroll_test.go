package rty

import (
	"strconv"
	"strings"
	"testing"
)

func TestElementScroll(t *testing.T) {
	f := newElementScrollTestFixture(t)

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

func TestTextScroll(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	sl := NewTextScrollLayout("foo")
	sl.Add(TextString(strings.Repeat("hiaeiurhgeiugheriuhgrtiuhgrtgn\n", 200)))
	i.Run("vertically overflowed box", 10, 10, sl)
}

type elementScrollTestFixture struct {
	i InteractiveTester
}

func newElementScrollTestFixture(t *testing.T) *elementScrollTestFixture {
	return &elementScrollTestFixture{
		i: NewInteractiveTester(t, screen),
	}
}

func (f *elementScrollTestFixture) layout() Component {
	var childrenNames []string
	for i := 0; i < 20; i++ {
		childrenNames = append(childrenNames, strconv.FormatInt(int64(i+1), 10))
	}

	l, selectedName := f.i.rty.RegisterElementScroll("items", childrenNames)

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
	f.i.Run(name, 20, 10, f.layout())
}

func (f *elementScrollTestFixture) down() {
	f.i.render(20, 10, f.layout())
	f.i.rty.ElementScroller("items").Down()
}

func (f *elementScrollTestFixture) up() {
	f.i.render(20, 10, f.layout())
	f.i.rty.ElementScroller("items").Up()
}
