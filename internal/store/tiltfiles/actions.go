package tiltfiles

import (
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type TiltfileUpsertAction struct {
	Tiltfile *v1alpha1.Tiltfile
}

func NewTiltfileUpsertAction(tf *v1alpha1.Tiltfile) TiltfileUpsertAction {
	return TiltfileUpsertAction{Tiltfile: tf.DeepCopy()}
}

func (TiltfileUpsertAction) Action() {}

type TiltfileDeleteAction struct {
	Name string
}

func (TiltfileDeleteAction) Action() {}
