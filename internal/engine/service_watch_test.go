package engine

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

func TestServiceWatch(t *testing.T) {
	f := newSWFixture(t)
	defer f.TearDown()

	f.addManifest("server")

	f.sw.OnChange(f.ctx, f.store)

	ls := k8s.TiltRunSelector()

	nodePort := 9998

	s := f.serviceNamed("foo", nodePort)
	f.kClient.EmitService(ls, s)

	expectedSCA := ServiceChangeAction{Service: s, URL: &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", f.nip, nodePort),
		Path:   "/",
	}}

	f.assertObservedServiceChangeActions(expectedSCA)
}

func (f *swFixture) addManifest(manifestName string) {
	state := f.store.LockMutableStateForTesting()
	state.WatchFiles = true
	dt := model.K8sTarget{Name: model.TargetName(manifestName)}
	m := model.Manifest{Name: model.ManifestName(manifestName)}.WithDeployTarget(dt)
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	f.store.UnlockMutableState()
}

func (f *swFixture) serviceNamed(name string, nodePort int) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 9998, NodePort: int32(nodePort)}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP:       string(f.nip),
						Hostname: string(f.nip),
					},
				},
			},
		},
	}
}

type swFixture struct {
	t          *testing.T
	kClient    *k8s.FakeK8sClient
	nip        k8s.NodeIP
	sw         *ServiceWatcher
	ctx        context.Context
	cancel     func()
	store      *store.Store
	getActions func() []store.Action
}

func newSWFixture(t *testing.T) *swFixture {
	kClient := k8s.NewFakeK8sClient()

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	nip := k8s.NodeIP("fakeip")

	ret := &swFixture{
		kClient: kClient,
		sw:      NewServiceWatcher(kClient, nip),
		nip:     nip,
		ctx:     ctx,
		cancel:  cancel,
		t:       t,
	}

	ret.store, ret.getActions = store.NewStoreForTesting()
	go ret.store.Loop(ctx)

	return ret
}

func (f *swFixture) TearDown() {
	f.kClient.TearDown()
	f.cancel()
}

func (f *swFixture) assertObservedServiceChangeActions(expectedSCAs ...ServiceChangeAction) {
	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		actions := f.getActions()
		if len(actions) == len(expectedSCAs) {
			break
		}
	}

	var observedSCAs []ServiceChangeAction
	for _, a := range f.getActions() {
		sca, ok := a.(ServiceChangeAction)
		if !ok {
			f.t.Fatalf("got non-%T: %v", ServiceChangeAction{}, a)
		}
		observedSCAs = append(observedSCAs, sca)
	}
	if !assert.Equal(f.t, expectedSCAs, observedSCAs) {
		f.t.FailNow()
	}
}
