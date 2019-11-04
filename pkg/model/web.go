package model

import (
	"flag"
	"fmt"
	"net/url"

	"github.com/spf13/pflag"
)

type NoBrowser bool // flag for disabling automatic browser opening

// Web version of the form vA.B.C, where A, B, and C are integers
// Used for fetching web assets
type WebVersion string

// Web version of the form aaaaaaa where a is a hex letter
// Used for fetching web assets
type WebSHA string

// Mode for developing Tilt web UX.
//
// Currently controls whether we use production asset bundles (JS/CSS)
// or local hot-reloaded asset bundles.

type WebMode string

const (
	// By default, we serve the js locally in dev builds and from prod in released
	// builds.
	DefaultWebMode WebMode = "default"

	// Local webpack server
	LocalWebMode WebMode = "local"

	// Prod gcloud bucket
	ProdWebMode WebMode = "prod"

	// Precompiled with `make build-js`. This is an experimental mode
	// we're playing around with to avoid the cost of webpack startup.
	PrecompiledWebMode WebMode = "precompiled"
)

func (m *WebMode) String() string {
	return string(*m)
}

func (m *WebMode) Set(v string) error {
	switch v {
	case string(DefaultWebMode):
		*m = DefaultWebMode
	case string(PrecompiledWebMode):
		*m = PrecompiledWebMode
	case string(LocalWebMode):
		*m = LocalWebMode
	case string(ProdWebMode):
		*m = ProdWebMode
	default:
		return UnrecognizedWebModeError(v)
	}
	return nil
}

func (m *WebMode) Type() string {
	return "WebMode"
}

func UnrecognizedWebModeError(v string) error {
	return fmt.Errorf("Unrecognized web mode: %s. Allowed values: %s", v, []WebMode{
		DefaultWebMode, LocalWebMode, ProdWebMode, PrecompiledWebMode,
	})
}

var emptyWebMode = WebMode("")
var _ flag.Value = &emptyWebMode
var _ pflag.Value = &emptyWebMode

type WebHost string
type WebPort int
type WebDevPort int
type WebURL url.URL

func (u WebURL) String() string {
	url := (*url.URL)(&u)
	return url.String()
}

func (u WebURL) Empty() bool {
	return WebURL{} == u
}
