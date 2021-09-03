package user

// TODO(nick): Eventually would like this interface to help with
// 1) other kinds of user preferences (token? analytics opt-in/opt-out?)
// 2) server-based preferences
type Prefs struct {
}

// Read/write metrics setting from a store.
// Inspired loosely by https://pkg.go.dev/k8s.io/client-go/kubernetes/typed/core/v1#PodInterface
type PrefsInterface interface {
	Get() (Prefs, error)
	Update(newPrefs Prefs) error
}
