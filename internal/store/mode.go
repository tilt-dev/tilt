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

	// "CI" is a mode that builds and applies all resources,
	// waits until they come up, then exits.
	EngineModeCI = EngineMode{Name: "ci"}
)

func (m EngineMode) WatchesFiles() bool {
	return m == EngineModeUp
}

func (m EngineMode) WatchesRuntime() bool {
	return m == EngineModeUp || m == EngineModeCI
}

func (m EngineMode) IsApplyMode() bool {
	return m == EngineModeApply
}

func (m EngineMode) IsCIMode() bool {
	return m == EngineModeCI
}
