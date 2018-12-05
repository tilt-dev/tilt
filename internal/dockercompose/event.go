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

type Action string

const (
	// Add 'actions' here (and to `UnmarshalJSON` below`) as we support them

	// CONTAINER actions
	ActionAttach     Action = "attach"
	ActionCreate     Action = "create"
	ActionDie        Action = "die"
	ActionExecDie    Action = "exec_die"
	ActionExecAttach Action = "exec_attach"
	ActionExecCreate Action = "exec_create"
	ActionKill       Action = "kill"
	ActionRename     Action = "rename"
	ActionRestart    Action = "restart"
	ActionStart      Action = "start"
	ActionStop       Action = "stop"
	ActionUpdate     Action = "update"
)

func (a *Action) UnmarshalJSON(b []byte) error {
	s := string(b)
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = unquoted
	}

	if s == "attach" {
		*a = ActionAttach
	} else if s == "create" {
		*a = ActionCreate
	} else if s == "die" {
		*a = ActionDie
	} else if s == "exec_attach" {
		*a = ActionExecAttach
	} else if s == "exec_die" {
		*a = ActionExecDie
	} else if s == "exec_create" {
		*a = ActionExecCreate
	} else if s == "kill" {
		*a = ActionKill
	} else if s == "rename" {
		*a = ActionRename
	} else if s == "restart" {
		*a = ActionRestart
	} else if s == "start" {
		*a = ActionStart
	} else if s == "stop" {
		*a = ActionStop
	} else if s == "update" {
		*a = ActionUpdate
	} else {
		return fmt.Errorf("unknown `Action` in docker-compose event: %s", s)
	}
	return nil
}
