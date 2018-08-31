package engine

import (
	"fmt"
	"strings"
)

type BrowserMode string

const (
	BrowserAuto BrowserMode = "auto"
	BrowserOff              = "off"
)

func (m BrowserMode) String() string {
	return string(m)
}

func (m BrowserMode) Type() string {
	return "BrowserMode"
}

func (m *BrowserMode) Set(val string) error {
	*m = BrowserMode(strings.ToLower(val))
	switch *m {
	case BrowserAuto, BrowserOff:
		return nil
	default:
		return fmt.Errorf("Invalid BrowserMode. Valid values: %s, %s", BrowserAuto, BrowserOff)
	}
}
