package build

type Label string
type LabelValue string
type Labels map[Label]LabelValue

const (
	// Label for all image builds created with Tilt.
	//
	// It's the responsibility of ImageBuilder to ensure
	// that all images built with Tilt have an appropriate BuildMode label.
	BuildMode Label = "tilt.buildMode"

	// Label when an image is created by a test.
	TestImage = "tilt.test"
)

const (
	BuildModeScratch  LabelValue = "scratch"
	BuildModeExisting            = "existing"
)
