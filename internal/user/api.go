package user

import "github.com/tilt-dev/tilt/pkg/model"

// TODO(nick): Eventually would like this interface to help with
// 1) other kinds of user preferences (token? analytics opt-in/opt-out?)
// 2) server-based preferences
type Prefs struct {
	// The kind of metrics stack the user is talking to.
	MetricsMode model.MetricsMode `json:"metricsMode,omitempty" yaml:"metricsMode,omitempty"`
}

// Read/write metrics setting from a store.
// Inspired loosely by https://pkg.go.dev/k8s.io/client-go/kubernetes/typed/core/v1#PodInterface
type PrefsInterface interface {
	Get() (Prefs, error)
	Update(newPrefs Prefs) error
}
