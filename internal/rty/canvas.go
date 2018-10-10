package rty

import (
	"fmt"

	"github.com/windmilleng/tcell"
)

// Canvases hold content.

type Canvas interface {
	Size() (int, int)
	SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) error
	Close() (int, int)
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

type lineRange struct {
	start int
	end   int
}

func newTempCanvas(width, height int) *TempCanvas {
	var cells [][]cell
	if height != GROW {
		cells = make([][]cell, height)
		for i := 0; i < height; i++ {
			cells[i] = make([]cell, width)
		}
	}
	return &TempCanvas{width: width, height: height, cells: cells}
}

func (c *TempCanvas) Size() (int, int) {
	return c.width, c.height
}

func (c *TempCanvas) Close() (int, int) {
	if c.height == GROW {
		c.height = len(c.cells)
	}
	return c.width, c.height
}

func (c *TempCanvas) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) error {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		panic(fmt.Errorf("cell %v,%v outside canvas %v,%v", x, y, c.width, c.height))
	}

	for y >= len(c.cells) {
		c.cells = append(c.cells, make([]cell, c.width))
	}

	c.cells[y][x] = cell{ch: mainc, style: style}
	return nil
}

func (c *TempCanvas) GetContent(x, y int) (mainc rune, combc []rune, style tcell.Style, width int) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		panic(fmt.Errorf("cell %d, %d outside bounds %d, %d", x, y, c.width, c.height))
	}

	if y >= len(c.cells) {
		return 0, nil, tcell.StyleDefault, 1
	}

	cell := c.cells[y][x]
	return cell.ch, nil, cell.style, 1
}

type SubCanvas struct {
	del       Canvas
	startX    int
	startY    int
	width     int
	height    int
	highWater int
}

func newSubCanvas(del Canvas, startX int, startY int, width int, height int) *SubCanvas {
	_, delHeight := del.Size()
	if height == GROW && delHeight != GROW {
		panic(fmt.Errorf("can't create a growing subcanvas from a non-growing subcanvas"))
	}
	return &SubCanvas{
		del:       del,
		startX:    startX,
		startY:    startY,
		width:     width,
		height:    height,
		highWater: -1,
	}
}

func (c *SubCanvas) Size() (int, int) {
	return c.width, c.height
}

func (c *SubCanvas) Close() (int, int) {
	if c.height == GROW {
		c.height = c.highWater + 1
	}

	return c.width, c.height
}

func (c *SubCanvas) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) error {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return fmt.Errorf("coord %d,%d is outside bounds %d,%d", x, y, c.width, c.height)
	}

	if c.height == GROW && y > c.highWater {
		c.highWater = y
	}

	return c.del.SetContent(c.startX+x, c.startY+y, mainc, combc, style)
}

func (c *SubCanvas) GetContent(x int, y int) (rune, []rune, tcell.Style, int) {
	return c.del.GetContent(x, y)
}

type ScreenCanvas struct {
	del tcell.Screen
}

func newScreenCanvas(del tcell.Screen) *ScreenCanvas {
	return &ScreenCanvas{del: del}
}

func (c *ScreenCanvas) Size() (int, int) {
	return c.del.Size()
}

func (c *ScreenCanvas) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) error {
	c.del.SetContent(x, y, mainc, combc, style)
	return nil
}

func (c *ScreenCanvas) Close() (int, int) {
	return c.del.Size()
}

func (c *ScreenCanvas) GetContent(x, y int) (mainc rune, combc []rune, style tcell.Style, width int) {
	return c.del.GetContent(x, y)
}
