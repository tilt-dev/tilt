package uisession

import (
	"context"

	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type Subscriber struct {
	lastSession *v1alpha1.UISession
}

func NewSubscriber() *Subscriber {
	return &Subscriber{}
}

func (s *Subscriber) OnChange(ctx context.Context, store store.RStore, summary store.ChangeSummary) {
	if summary.IsLogOnly() {
		return
	}

	state := store.RLockState()
	session := webview.ToUISession(state)
	store.RUnlockState()

	exists := s.lastSession != nil
	if !exists {
		store.Dispatch(NewUISessionCreateAction(session))
		s.lastSession = session
		return
	}

	if !equality.Semantic.DeepEqual(session.Status, s.lastSession.Status) {
		store.Dispatch(NewUISessionUpdateStatusAction(session))
		s.lastSession = session
	}
}

var _ store.Subscriber = &Subscriber{}
