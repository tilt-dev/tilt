package dockercompose

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type Event struct {
	Time       string     `json:"time"` // todo: time
	Type       Type       `json:"type"`
	Action     string     `json:"action"`
	ID         string     `json:"id"` // todo: type?
	Service    string     `json:"service"`
	Attributes Attributes `json:"attributes"`
}

type Attributes struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

func EventFromJsonStr(j string) (Event, error) {
	var evt Event

	b := []byte(j)
	err := json.Unmarshal(b, &evt)

	return evt, err
}

// https://docs.docker.com/engine/reference/commandline/events/
type Type int

const (
	// Add 'types' here (and to `stringToType` below) as we support them
	TypeUnknown Type = iota
	TypeContainer
)

var stringToType = map[string]Type{
	"container": TypeContainer,
}

func (t Type) String() string {
	for str, typ := range stringToType {
		if typ == t {
			return str
		}
	}
	return "unknown"
}

func (t Type) MarshalJSON() ([]byte, error) {
	s := t.String()
	return []byte(fmt.Sprintf("%q", s)), nil
}

func (t *Type) UnmarshalJSON(b []byte) error {
	s := string(b)
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = unquoted
	}

	typ := stringToType[s] // if type not in map, this returns 0 (i.e. TypeUnknown)
	*t = typ
	return nil
}
