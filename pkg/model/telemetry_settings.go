package model

type TelemetrySettings struct {
	Cmd     Cmd
	Workdir string // directory from which this Cmd should be run
}
