package tiltfile

import (
	"context"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type FakeTiltfileLoader struct {
	Result   TiltfileLoadResult
	Args     []string
	Delegate TiltfileLoader
}

var _ TiltfileLoader = &FakeTiltfileLoader{}

func NewFakeTiltfileLoader() *FakeTiltfileLoader {
	return &FakeTiltfileLoader{}
}

func (tfl *FakeTiltfileLoader) Load(ctx context.Context, tf *v1alpha1.Tiltfile) TiltfileLoadResult {
	tfl.Args = tf.Spec.Args
	if tfl.Delegate != nil {
		return tfl.Delegate.Load(ctx, tf)
	}
	return tfl.Result
}

// the Args that was passed to the last invocation of Load
func (tfl *FakeTiltfileLoader) PassedArgs() []string {
	return tfl.Args
}
