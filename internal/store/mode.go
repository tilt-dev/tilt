package store

// Defines different executions modes for running Tilt,
// and deciding when to exit.
type EngineMode struct {
	Name string
}

var (
	// "up" is an interactive dev mode that watches files and resources.
	EngineModeUp = EngineMode{}

	// "apply" is a dev mode that builds and applies all resources,
	// but doesn't wait to see if they come up.
	EngineModeApply = EngineMode{Name: "apply"}
)

func (m EngineMode) WatchesFiles() bool {
	return m == EngineModeUp
}

func (m EngineMode) WatchesRuntime() bool {
	return m == EngineModeUp
}

func (m EngineMode) IsApplyMode() bool {
	return m == EngineModeApply
}
