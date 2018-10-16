package demo

import (
	"context"
	"fmt"

	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

// Runs the demo script
type Script struct {
	hud   hud.HeadsUpDisplay
	upper model.ManifestCreator
	env   k8s.Env
}

func NewScript(upper model.ManifestCreator, hud hud.HeadsUpDisplay, env k8s.Env) Script {
	return Script{
		upper: upper,
		hud:   hud,
		env:   env,
	}
}

func (s Script) Run(ctx context.Context) error {
	fmt.Println("TODO(nick): implement demo")
	return nil
}
