package model

import (
	"flag"
	"fmt"
	"net/url"

	"github.com/spf13/pflag"
)

// Web version of the form vA.B.C, where A, B, and C are integers.
// Used for fetching web assets
type WebVersion string

// Mode for developing Tilt web UX.
//
// Currently controls whether we use production asset bundles (JS/CSS)
// or local hot-reloaded asset bundles.

type WebMode string

const (
	LocalWebMode WebMode = "local"
	ProdWebMode  WebMode = "prod"
)

func (m *WebMode) String() string {
	return string(*m)
}

func (m *WebMode) Set(v string) error {
	switch v {
	case string(LocalWebMode):
		*m = LocalWebMode
	case string(ProdWebMode):
		*m = ProdWebMode
	default:
		return fmt.Errorf("Unknown dev mode: %s", v)
	}
	return nil
}

func (m *WebMode) Type() string {
	return "WebMode"
}

var emptyWebMode = WebMode("")
var _ flag.Value = &emptyWebMode
var _ pflag.Value = &emptyWebMode

type WebPort int
type WebURL url.URL

func (u WebURL) String() string {
	url := (*url.URL)(&u)
	return url.String()
}

func (u WebURL) Empty() bool {
	return WebURL{} == u
}
