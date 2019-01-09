package dockercompose

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type Event struct {
	Time       string     `json:"time"` // todo: time
	Type       Type       `json:"type"`
	Action     Action     `json:"action"`
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
type Type string

const (
	// Add 'types' here (and to `UnmarshalJSON` below) as we support them
	TypeContainer Type = "container"
)

func (t *Type) UnmarshalJSON(b []byte) error {
	s := string(b)
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = unquoted
	}

	if s == "container" {
		*t = TypeContainer
	} else {
		return fmt.Errorf("unknown `Type` in docker-compose event: %s", s)
	}
	return nil
}

type Action int

const (
	// Add 'actions' here (and to `stringToAction` below`) as we support them

	// CONTAINER actions
	ActionAttach = iota
	ActionCommit
	ActionCopy
	ActionCreate
	ActionDestroy
	ActionDie
	ActionExecAttach
	ActionExecCreate
	ActionExecDie
	ActionKill
	ActionRename
	ActionRestart
	ActionStart
	ActionStop
	ActionUpdate
)

var stringToAction = map[string]Action{
	"attach":      ActionAttach,
	"commit":      ActionCommit,
	"copy":        ActionCopy,
	"create":      ActionCreate,
	"destroy":     ActionDestroy,
	"die":         ActionDie,
	"exec_attach": ActionExecAttach,
	"exec_create": ActionExecCreate,
	"exec_die":    ActionExecDie,
	"kill":        ActionKill,
	"rename":      ActionRename,
	"restart":     ActionRestart,
	"start":       ActionStart,
	"stop":        ActionStop,
	"update":      ActionUpdate,
}

func (a *Action) UnmarshalJSON(b []byte) error {
	s := string(b)
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = unquoted
	}

	action, ok := stringToAction[s]
	if !ok {
		return fmt.Errorf("unknown `Action` in docker-compose event: %s", s)
	}
	*a = action
	return nil
}
