package rty

import "testing"

func TestBoxes(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	i.Run("10x10 box", 10, 10, NewGrowingBox())
	b := NewGrowingBox()
	b.SetInner(TextString("hello world"))
	i.Run("text in box", 20, 10, b)
	i.Run("wrapped text in box", 10, 10, b)
	b.SetTitle("so very important")
	i.Run("box with title", 20, 10, b)
	i.Run("box with short title", 5, 10, b)

	b = NewBox(TextString("hello world"))
	i.Run("non-growing box", 20, 20, b)
}

func TestWindows(t *testing.T) {
	i := NewInteractiveTester(t, screen)

	i.Run("10x10 window", 10, 10, NewGrowingWindow())
	window := NewGrowingWindow()
	window.SetInner(TextString("hello world"))
	i.Run("text in window", 20, 10, window)
	i.Run("wrapped text in window", 10, 10, window)
	window.SetTitle("so very important")
	i.Run("window with title", 20, 10, window)
	i.Run("window with short title", 5, 10, window)

	window = NewWindow(TextString("hello world"))
	i.Run("non-growing window", 20, 20, window)
}
