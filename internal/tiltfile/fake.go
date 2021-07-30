package tiltfile

import (
	"context"

	"github.com/tilt-dev/tilt/pkg/model"
)

type FakeTiltfileLoader struct {
	Result          TiltfileLoadResult
	userConfigState model.UserConfigState
	Delegate        TiltfileLoader
}

var _ TiltfileLoader = &FakeTiltfileLoader{}

func NewFakeTiltfileLoader() *FakeTiltfileLoader {
	return &FakeTiltfileLoader{}
}

func (tfl *FakeTiltfileLoader) Load(ctx context.Context, filename string, userConfigState model.UserConfigState) TiltfileLoadResult {
	tfl.userConfigState = userConfigState
	if tfl.Delegate != nil {
		return tfl.Delegate.Load(ctx, filename, userConfigState)
	}
	return tfl.Result
}

// the UserConfigState that was passed to the last invocation of Load
func (tfl *FakeTiltfileLoader) PassedUserConfigState() model.UserConfigState {
	return tfl.userConfigState
}
