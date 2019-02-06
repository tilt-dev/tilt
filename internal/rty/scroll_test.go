package rty

import (
	"fmt"
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

	f.bottom()
	f.run("jumped to bottom")
}

func TestElementScrollWrap(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	sl, _ := i.rty.RegisterElementScroll("baz", []string{"hi"})
	sl.Add(TextString(strings.Repeat("abcdefgh", 20)))
	i.Run("line wrapped element scroll", 10, 10, sl)
}

func TestElementScrollPerfectlyFilled(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	var names []string
	for j := 0; j < 10; j++ {
		names = append(names, fmt.Sprintf("%d", j+1))
	}

	sl, _ := i.rty.RegisterElementScroll("qux", names)
	for range names {
		sl.Add(TextString("abcd"))
	}
	i.Run("element scroll perfectly filled", 10, len(names), sl)
}

func TestTextScroll(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	sl := NewTextScrollLayout("foo")
	sl.Add(TextString(strings.Repeat("abcd\n", 200)))
	i.Run("vertically overflowed text scroll", 10, 10, sl)

	sl = NewTextScrollLayout("bar")
	sl.Add(TextString(strings.Repeat("abcd", 200)))
	i.Run("line wrapped text scroll", 10, 10, sl)

	sl = NewTextScrollLayout("bar2")
	s := ""
	for i := 1; i <= 20; i++ {
		s = s + fmt.Sprintf("%d\n", i)
	}
	sl.Add(TextString(s))
	ts := i.rty.TextScroller(sl.name)
	ts.Bottom()
	ts.Up()
	for i := 0; i < 5; i++ {
		ts.Down()
		ts.Down()
	}
	i.Run("textscroll stop scrolling at bottom", 10, 10, sl)
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
			c.Add(TextString(strings.Repeat(n, 9)))
		}
		components = append(components, c)
	}

	for _, c := range components {
		l.Add(c)
	}

	return l
}

func (f *elementScrollTestFixture) scroller() ElementScroller {
	return f.i.rty.ElementScroller("items")
}

func (f *elementScrollTestFixture) run(name string) {
	f.i.Run(name, 20, 10, f.layout())
}

func (f *elementScrollTestFixture) down() {
	f.i.render(20, 10, f.layout())
	f.scroller().Down()
}

func (f *elementScrollTestFixture) up() {
	f.i.render(20, 10, f.layout())
	f.scroller().Up()
}

func (f *elementScrollTestFixture) bottom() {
	f.i.render(20, 10, f.layout())
	f.scroller().Bottom()
}
