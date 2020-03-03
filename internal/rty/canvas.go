package rty

import (
	"fmt"

	"github.com/gdamore/tcell"
)

// Canvases hold content.

type Canvas interface {
	Size() (int, int)
	SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style)
	Close() (int, int)
	GetContent(x, y int) (mainc rune, combc []rune, style tcell.Style, width int)
}

func totalHeight(canvases []Canvas) int {
	total := 0
	for _, c := range canvases {
		_, h := c.Size()
		total += h
	}
	return total
}

// Implementations below
type cell struct {
	ch    rune
	style tcell.Style
}

type TempCanvas struct {
	width   int
	height  int
	cells   [][]cell
	style   tcell.Style
	handler ErrorHandler
}

var _ Canvas = &TempCanvas{}

func newTempCanvas(width, height int, style tcell.Style, handler ErrorHandler) *TempCanvas {
	c := &TempCanvas{width: width, height: height, handler: handler}
	if height != GROW {
		c.cells = make([][]cell, height)
		for i := 0; i < height; i++ {
			c.cells[i] = c.makeRow()
		}
	}
	return c
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

func (c *TempCanvas) makeRow() []cell {
	row := make([]cell, c.width)
	for i := 0; i < c.width; i++ {
		row[i].style = c.style
	}
	return row
}

func (c *TempCanvas) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if mainc == 0 {
		mainc = ' '
	}
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		c.handler.Errorf("cell %v,%v outside canvas %v,%v", x, y, c.width, c.height)
		return
	}

	for y >= len(c.cells) {
		c.cells = append(c.cells, c.makeRow())
	}

	c.cells[y][x] = cell{ch: mainc, style: style}
}

func (c *TempCanvas) GetContent(x, y int) (mainc rune, combc []rune, style tcell.Style, width int) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		c.handler.Errorf("cell %d, %d outside bounds %d, %d", x, y, c.width, c.height)
		return 0, nil, tcell.StyleDefault, 1
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
	style     tcell.Style
	needsFill bool
	handler   ErrorHandler
}

func newSubCanvas(del Canvas, startX int, startY int, width int, height int, style tcell.Style, handler ErrorHandler) (*SubCanvas, error) {
	_, delHeight := del.Size()
	if height == GROW && delHeight != GROW {
		return nil, fmt.Errorf("can't create a growing subcanvas from a non-growing subcanvas")
	}

	needsFill := true
	delSubCanvas, ok := del.(*SubCanvas)
	if ok {
		// If this is a subcanvas of a subcanvas with the exact same style (or with
		// only the foreground different), we already reset the canvas to the
		// current style. No need to re-fill.
		needsFill = style.Foreground(tcell.ColorDefault) !=
			delSubCanvas.style.Foreground(tcell.ColorDefault)
	}

	r := &SubCanvas{
		del:       del,
		startX:    startX,
		startY:    startY,
		width:     width,
		height:    height,
		highWater: -1,
		style:     style,
		needsFill: needsFill,
		handler:   handler,
	}
	if needsFill {
		r.fill(-1)
	}
	return r, nil
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

func (c *SubCanvas) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if mainc == 0 {
		mainc = ' '
	}
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		c.handler.Errorf("coord %d,%d is outside bounds %d,%d", x, y, c.width, c.height)
		return
	}

	if c.height == GROW && y > c.highWater {
		oldHighWater := c.highWater
		c.highWater = y
		if c.needsFill {
			c.fill(oldHighWater)
		}
	}
	c.del.SetContent(c.startX+x, c.startY+y, mainc, combc, style)
}

func (c *SubCanvas) fill(lastFilled int) {
	startY := lastFilled + 1
	maxY := c.height
	if maxY == GROW {
		maxY = c.highWater + 1
	}
	for y := startY; y < maxY; y++ {
		for x := 0; x < c.width; x++ {
			c.del.SetContent(c.startX+x, c.startY+y, ' ', nil, c.style)
		}
	}
}

func (c *SubCanvas) GetContent(x int, y int) (rune, []rune, tcell.Style, int) {
	return c.del.GetContent(x, y)
}

type ScreenCanvas struct {
	del     tcell.Screen
	handler ErrorHandler
}

var _ Canvas = &ScreenCanvas{}

func newScreenCanvas(del tcell.Screen, handler ErrorHandler) *ScreenCanvas {
	return &ScreenCanvas{del: del, handler: handler}
}

func (c *ScreenCanvas) Size() (int, int) {
	return c.del.Size()
}

func (c *ScreenCanvas) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if mainc == 0 {
		mainc = ' '
	}
	c.del.SetContent(x, y, mainc, combc, style)
}

func (c *ScreenCanvas) Close() (int, int) {
	return c.del.Size()
}

func (c *ScreenCanvas) GetContent(x, y int) (mainc rune, combc []rune, style tcell.Style, width int) {
	return c.del.GetContent(x, y)
}
