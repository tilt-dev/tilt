package user

import "github.com/tilt-dev/tilt/pkg/model"

func UpdateMetricsMode(pi PrefsInterface, mode model.MetricsMode) error {
	prefs, err := pi.Get()
	if err != nil {
		return err
	}
	prefs.MetricsMode = mode
	return pi.Update(prefs)
}
