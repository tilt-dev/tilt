// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package tui

import "strconv"

func Chip(fg, bg int, content string) string {
	fgAnsi := "\x1b[38;5;" + strconv.Itoa(fg) + "m"
	bgAnsi := "\x1b[48;5;" + strconv.Itoa(bg) + "m"
	return fgAnsi + bgAnsi + content + "\x1b[0m"
}
