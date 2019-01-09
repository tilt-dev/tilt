package dockercompose

import (
	"encoding/json"
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
type Type int

const (
	// Add 'types' here (and to `stringToType` below) as we support them
	TypeUnknown = iota
	TypeContainer
)

var stringToType = map[string]Type{
	"container": TypeContainer,
}

func (t *Type) UnmarshalJSON(b []byte) error {
	s := string(b)
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = unquoted
	}

	evtType := stringToType[s]
	*t = evtType
	return nil
}

type Action int

const (
	// Add 'actions' here (and to `stringToAction` below`) as we support them

	// CONTAINER actions
	ActionUnknown = iota
	ActionAttach
	ActionCommit
	ActionCopy
	ActionCreate
	ActionDestroy
	ActionDie
	ActionExecCreate
	ActionExecDetach
	ActionExecDie
	ActionExecStart
	ActionExport
	ActionHealthStatus
	ActionKill
	ActionOom
	ActionPause
	ActionRename
	ActionResize
	ActionRestart
	ActionStart
	ActionStop
	ActionTop
	ActionUnpause
	ActionUpdate
)

var stringToAction = map[string]Action{
	"attach":        ActionAttach,
	"commit":        ActionCommit,
	"copy":          ActionCopy,
	"create":        ActionCreate,
	"destroy":       ActionDestroy,
	"die":           ActionDie,
	"exec_create":   ActionExecCreate,
	"exec_detach":   ActionExecDetach,
	"exec_die":      ActionExecDie,
	"exec_start":    ActionExecStart,
	"export":        ActionExport,
	"health_status": ActionHealthStatus,
	"kill":          ActionKill,
	"oom":           ActionOom,
	"pause":         ActionPause,
	"rename":        ActionRename,
	"resize":        ActionResize,
	"restart":       ActionRestart,
	"start":         ActionStart,
	"stop":          ActionStop,
	"top":           ActionTop,
	"unpause":       ActionUnpause,
	"update":        ActionUpdate,
}

func (a *Action) UnmarshalJSON(b []byte) error {
	s := string(b)
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = unquoted
	}

	action := stringToAction[s]
	*a = action
	return nil
}
