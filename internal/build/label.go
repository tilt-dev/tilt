package build

import "github.com/windmilleng/tilt/internal/dockerfile"

const (
	// Label for all image builds created with Tilt.
	//
	// It's the responsibility of ImageBuilder to ensure
	// that all images built with Tilt have an appropriate BuildMode label.
	BuildMode dockerfile.Label = "tilt.buildMode"

	// Label when an image is created by a test.
	TestImage dockerfile.Label = "tilt.test"

	// Label when an image is for path caching.
	CacheImage dockerfile.Label = "tilt.cache"
)

const (
	BuildModeScratch dockerfile.LabelValue = "scratch"
)
