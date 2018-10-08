package rty

import (
	"fmt"

	"github.com/windmilleng/tcell"
)

// Canvases hold content.

type CanvasWriter interface {
	Size() (int, int)
	SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style)
	Sub(startX, startY, width, height int) CanvasWriter
}

type CanvasReader interface {
	Size() (int, int)
	GetContent(x, y int) (mainc rune, combc []rune, style tcell.Style, width int)
}

// Implementations below
type cell struct {
	ch    rune
	style tcell.Style
}

type TempCanvas struct {
	width  int
	height int
	cells  [][]cell
}

func NewTempCanvas(width, height int) *TempCanvas {
	cells := make([][]cell, height)
	for i := 0; i < height; i++ {
		cells[i] = make([]cell, width)
	}
	return &TempCanvas{width: width, height: height, cells: cells}
}

func (c *TempCanvas) Size() (int, int) {
	return c.width, c.height
}

func (c *TempCanvas) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if x < 0 || x >= c.width || y < 0 || y >= c.width {
		return
	}

	c.cells[y][x] = cell{ch: mainc, style: style}
}

func (c *TempCanvas) GetContent(x, y int) (mainc rune, combc []rune, style tcell.Style, width int) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		panic(fmt.Errorf("cell %d, %d outside bounds %d, %d", x, y, c.width, c.height))
	}

	cell := c.cells[y][x]
	return cell.ch, nil, cell.style, 1
}

func (c *TempCanvas) Sub(startX, startY, width, height int) CanvasWriter {
	return newSubCanvas(c, startX, startY, width, height)
}

type SubCanvas struct {
	del    CanvasWriter
	startX int
	startY int
	width  int
	height int
}

func newSubCanvas(del CanvasWriter, startX int, startY int, width int, height int) *SubCanvas {
	return &SubCanvas{
		del:    del,
		startX: startX,
		startY: startY,
		width:  width,
		height: height,
	}
}

func (c *SubCanvas) Size() (int, int) {
	return c.width, c.height
}

func (c *SubCanvas) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if x < 0 || x >= c.width || y < 0 || y >= c.width {
		return
	}

	c.del.SetContent(c.startX+x, c.startY+y, mainc, combc, style)
}

func (c *SubCanvas) Sub(startX, startY, width, height int) CanvasWriter {
	return newSubCanvas(c, startX, startY, width, height)
}

func Copy(src CanvasReader, dst CanvasWriter) {
	width, height := dst.Size()
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			mainc, combc, style, _ := src.GetContent(x, y)
			dst.SetContent(x, y, mainc, combc, style)
		}
	}
}

type ScreenCanvas struct {
	tcell.Screen
}

func NewScreenCanvas(s tcell.Screen) *ScreenCanvas {
	return &ScreenCanvas{s}
}

func (c *ScreenCanvas) Sub(startX, startY, width, height int) CanvasWriter {
	return newSubCanvas(c, startX, startY, width, height)
}
