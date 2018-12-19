package engine

import (
	"github.com/windmilleng/tilt/internal/container"

	"github.com/windmilleng/tilt/internal/k8s"
)

const pod1 = k8s.PodID("pod1")

var image1 = container.MustParseNamedTagged("re.po/project/myapp:tilt-936a185caaa266bb")

const digest1 = "sha256:936a185caaa266bb9cbe981e9e05cb78cd732b0b3280eb944412bb6f8f8f07af"
