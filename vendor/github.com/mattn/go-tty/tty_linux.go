//go:build linux
// +build linux

package tty

import (
	"golang.org/x/sys/unix"
)

const (
	ioctlReadTermios  = unix.TCGETS
	ioctlWriteTermios = unix.TCSETS
)
