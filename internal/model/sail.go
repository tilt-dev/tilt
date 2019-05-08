package model

import (
	"fmt"
	"net/url"
)

// Mode for developing Tilt against the Sail server.
//
// Currently controls whether we use a local instance or the production instance.
type SailMode string

const (
	SailModeDefault SailMode = "default"

	SailModeDisabled SailMode = "none"

	// Local sail server on localhost:10350
	SailModeLocal SailMode = "local"

	// Production sail server at sail.tilt.dev
	SailModeProd SailMode = "prod"
)

func (m *SailMode) String() string {
	return string(*m)
}

func (m *SailMode) Set(v string) error {
	switch v {
	case string(SailModeDefault):
		*m = SailModeDefault
	case string(SailModeDisabled):
		*m = SailModeDisabled
	case string(SailModeLocal):
		*m = SailModeLocal
	case string(SailModeProd):
		*m = SailModeProd
	default:
		return UnrecognizedWebModeError(v)
	}
	return nil
}

func (m *SailMode) Type() string {
	return "SailMode"
}

func (m *SailMode) IsEnabled() bool {
	mode := *m
	return mode == SailModeLocal || mode == SailModeProd
}

func UnrecognizedSailModeError(v string) error {
	return fmt.Errorf("Unrecognized sail mode: %s. Allowed values: %s", v, []SailMode{
		SailModeDefault, SailModeDisabled, SailModeLocal, SailModeProd,
	})
}

const DefaultSailPort = 10450

const (
	SailSecretKey = "Secret"
	SailRoomIDKey = "room_id"
)

type RoomID string

type SailRoomInfo struct {
	RoomID RoomID `json:"room_id"`
	Secret string `json:"secret"`
}

type SailNewRoomRequest struct {
	WebVersion WebVersion `json:"web_version"`
}

type SailPort int
type SailURL url.URL

func (u SailURL) Hostname() string {
	url := (*url.URL)(&u)
	return url.Hostname()
}

func (u SailURL) String() string {
	url := (*url.URL)(&u)
	return url.String()
}

func (u SailURL) Http() SailURL {
	if u.Hostname() == "localhost" {
		u.Scheme = "http"
	} else {
		u.Scheme = "https"
	}
	return u
}

func (u SailURL) Ws() SailURL {
	if u.Hostname() == "localhost" {
		u.Scheme = "ws"
	} else {
		u.Scheme = "wss"
	}
	return u
}

func (u SailURL) WithQueryParam(key, value string) SailURL {
	url := (*url.URL)(&u)
	q := url.Query()
	q.Set(key, value)
	url.RawQuery = q.Encode()
	return SailURL(*url)
}

func (u SailURL) Empty() bool {
	return SailURL{} == u
}
