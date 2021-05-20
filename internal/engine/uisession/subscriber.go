package uisession

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type Subscriber struct {
	client ctrlclient.Client
}

func NewSubscriber(client ctrlclient.Client) *Subscriber {
	return &Subscriber{client: client}
}

func (s *Subscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if summary.IsLogOnly() {
		return
	}

	state := st.RLockState()
	session := webview.ToUISession(state)
	st.RUnlockState()

	stored := &v1alpha1.UISession{}
	err := s.client.Get(ctx, types.NamespacedName{Name: session.Name}, stored)
	if apierrors.IsNotFound(err) {
		err := s.client.Create(ctx, session)
		if err != nil {
			logger.Get(ctx).Infof("creating uisession: %v", err)
			return
		}
		st.Dispatch(NewUISessionCreateAction(session))
		return
	} else if err != nil {
		logger.Get(ctx).Infof("fetching uisession: %v", err)
		return
	}

	if !apicmp.DeepEqual(session.Status, stored.Status) {
		update := &v1alpha1.UISession{
			ObjectMeta: *stored.ObjectMeta.DeepCopy(),
			Spec:       *stored.Spec.DeepCopy(),
			Status:     *session.Status.DeepCopy(),
		}
		err = s.client.Status().Update(ctx, update)
		if err != nil {
			logger.Get(ctx).Infof("updating uisession: %v", err)
			return
		}
		st.Dispatch(NewUISessionUpdateStatusAction(session))
	}
}

var _ store.Subscriber = &Subscriber{}
