package k8swatch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/fake"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/podbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestManifestsWithNoWatches(t *testing.T) {
	f := newMSFixture(t)

	state := f.store.LockMutableStateForTesting()
	m1 := manifestbuilder.New(f, "local").
		WithLocalServeCmd("echo hi").
		Build()
	state.UpsertManifestTarget(store.NewManifestTarget(m1))

	m2 := manifestbuilder.New(f, "dc").
		WithDockerCompose().
		Build()
	state.UpsertManifestTarget(store.NewManifestTarget(m2))

	// this is hacky - it's technically a K8s resource but has empty YAML, so there's nothing to watch
	// and the expectation is that no entity will be created the same as if it was a non-K8s resource
	m3 := manifestbuilder.New(f, "k8s-norefs").WithK8sYAML(" ").Build()
	state.UpsertManifestTarget(store.NewManifestTarget(m3))

	f.store.UnlockMutableState()

	f.ms.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	require.Never(t, func() bool {
		state := f.store.RLockState()
		defer f.store.RUnlockState()
		return len(state.KubernetesDiscoveries) != 0
	}, 500*time.Millisecond, 10*time.Millisecond,
		"No KubernetesDiscovery objects should have been created in store")
}

func TestK8sResources(t *testing.T) {
	type tc struct {
		name  string
		cfgNS k8s.Namespace
	}

	tcs := []tc{
		{name: "NoCfgNS", cfgNS: ""},
		{name: "DefaultK8sCfgNS", cfgNS: k8s.DefaultNamespace},
		{name: "CustomCfgNS", cfgNS: "custom-namespace"},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			f := newMSFixture(t)
			f.ms.cfgNS = tc.cfgNS

			m1 := f.upsertManifest("default-ns", testyaml.SanchoYAML)
			m2 := f.upsertManifest("explicit-ns", testyaml.DoggosDeploymentYaml)
			// the namespaces should have watch def, but there's no UID/name yet since nothing is deployed
			f.requireWatchRefs(m1.Name, watchRef(f.ms.cfgNS, "", ""))
			f.requireWatchRefs(m2.Name, watchRef("the-dog-zone", "", ""))

			// podbuilder can't automagically determine the context namespace, so we need to provide it
			deployment1 := podbuilder.New(t, m1).
				WithPodName("pod1").
				WithContextNamespace(tc.cfgNS).
				ObjectTreeEntities().
				Deployment()
			f.addDeployedEntity(m1, deployment1)
			f.requireWatchRefs(m1.Name, watchRef(f.ms.cfgNS, deployment1.UID(), deployment1.Name()))
			// m2 should not have changed yet
			f.requireWatchRefs(m2.Name, watchRef("the-dog-zone", "", ""))

			// podbuilder will find the namespace from the manifest YAML since the entities explicitly define one
			deployment2 := podbuilder.New(t, m2).WithPodName("pod2").ObjectTreeEntities().Deployment()
			f.addDeployedEntity(m2, deployment2)
			f.requireWatchRefs(m2.Name, watchRef("the-dog-zone", deployment2.UID(), deployment2.Name()))
			// m1 should have remained the same
			f.requireWatchRefs(m1.Name, watchRef(f.ms.cfgNS, deployment1.UID(), deployment1.Name()))
		})
	}
}

func TestExtraSelectors(t *testing.T) {
	f := newMSFixture(t)

	mn := model.ManifestName("extra")
	m := f.upsertManifest(mn, testyaml.SanchoYAML,
		// label order within a set must not matter - K8s will sort + guarantee consistent ordering
		labels.Set{"label2": "value2", "label1": "value1"},
		labels.Set{"label3": "value3"})
	// the namespace should have a watch def (or else we'd never observe anything to match labels against)
	f.requireWatchRefs(mn, watchRef(f.ms.cfgNS, "", ""))
	f.requireExtraSelectors(m.Name,
		labels.Set{"label1": "value1", "label2": "value2"},
		labels.Set{"label3": "value3"})

	f.upsertManifest(mn, testyaml.SanchoYAML, labels.Set{"label3": "value4"})
	f.requireExtraSelectors(mn, labels.Set{"label3": "value4"})
	// namespace watch should still exist
	f.requireWatchRefs(mn, watchRef(f.ms.cfgNS, "", ""))

	// remove all extra selectors
	f.upsertManifest(mn, testyaml.SanchoYAML)
	f.requireExtraSelectors(mn)
	// namespace watch should still exist
	f.requireWatchRefs(mn, watchRef(f.ms.cfgNS, "", ""))
}

func TestMultipleManifestsSameEntity(t *testing.T) {
	f := newMSFixture(t)

	m1 := f.upsertManifest("m1", testyaml.SanchoYAML)
	m2 := f.upsertManifest("m2", testyaml.SanchoYAML)
	// the namespaces should have watch def, but there's no UID/name yet since nothing is deployed
	f.requireWatchRefs(m1.Name, watchRef(k8s.DefaultNamespace, "", ""))
	f.requireWatchRefs(m2.Name, watchRef(k8s.DefaultNamespace, "", ""))

	deployment1 := podbuilder.New(t, m1).
		WithPodName("pod1").
		ObjectTreeEntities().
		Deployment()
	f.addDeployedEntity(m1, deployment1)
	f.requireWatchRefs(m1.Name, watchRef(k8s.DefaultNamespace, deployment1.UID(), deployment1.Name()))
	// m2 should not have changed yet
	f.requireWatchRefs(m2.Name, watchRef(k8s.DefaultNamespace, "", ""))

	deployment2 := podbuilder.New(t, m2).
		WithPodName("pod2").
		ObjectTreeEntities().
		Deployment()
	f.addDeployedEntity(m2, deployment2)
	f.requireWatchRefs(m2.Name, watchRef(k8s.DefaultNamespace, deployment2.UID(), deployment2.Name()))
	// m1 should have remained the same
	f.requireWatchRefs(m1.Name, watchRef(k8s.DefaultNamespace, deployment1.UID(), deployment1.Name()))
}

type msFixture struct {
	*tempdir.TempDirFixture
	t     testing.TB
	ctx   context.Context
	mu    sync.Mutex
	store *store.Store
	cli   ctrlclient.Client
	ms    *ManifestSubscriber
	cs    *changeSubscriber
}

func newMSFixture(t testing.TB) *msFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	cli := fake.NewTiltClient()
	ms := NewManifestSubscriber(k8s.DefaultNamespace, cli)
	cs := newChangeSubscriber(t)

	f := &msFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
		cli:            cli,
		ms:             ms,
		cs:             cs,
	}
	t.Cleanup(f.TearDown)

	st := store.NewStore(f.reducer, false)
	require.NoError(t, st.AddSubscriber(ctx, ms))
	require.NoError(t, st.AddSubscriber(ctx, cs))

	f.store = st

	go func() {
		err := st.Loop(ctx)
		testutils.FailOnNonCanceledErr(t, err, "store.Loop failed")
	}()

	return f
}

func (f *msFixture) reducer(ctx context.Context, state *store.EngineState, action store.Action) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch a := action.(type) {
	case KubernetesDiscoveryCreateAction:
		HandleKubernetesDiscoveryCreateAction(ctx, state, a)
	case KubernetesDiscoveryUpdateAction:
		HandleKubernetesDiscoveryUpdateAction(ctx, state, a)
	case KubernetesDiscoveryDeleteAction:
		HandleKubernetesDiscoveryDeleteAction(ctx, state, a)
	case store.PanicAction:
		f.t.Fatalf("Store received PanicAction: %v", a.Err)
	default:
		f.t.Fatalf("Unexpected action type: %T", action)
	}
}

func (f *msFixture) upsertManifest(mn model.ManifestName, yaml string, ls ...labels.Set) model.Manifest {
	f.t.Helper()
	state := f.store.LockMutableStateForTesting()
	m := manifestbuilder.New(f, mn).
		WithK8sYAML(yaml).
		WithK8sPodSelectors(ls).
		Build()
	mt := store.NewManifestTarget(m)
	state.UpsertManifestTarget(mt)
	f.store.UnlockMutableState()

	f.ms.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	// verify that a change summary was properly generated
	f.cs.waitForChangeAndReset(KeyForManifest(mn))

	return mt.Manifest
}

func (f *msFixture) addDeployedEntity(m model.Manifest, entity k8s.K8sEntity) {
	f.t.Helper()

	state := f.store.LockMutableStateForTesting()
	mState, ok := state.ManifestState(m.Name)
	if !ok {
		f.t.Fatalf("Unknown manifest: %s", m.Name)
	}

	runtimeState := mState.K8sRuntimeState()
	runtimeState.DeployedEntities = k8s.ObjRefList{entity.ToObjectReference()}
	mState.RuntimeState = runtimeState
	f.store.UnlockMutableState()

	f.ms.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
}

func (f *msFixture) requireDiscoveryState(mn model.ManifestName, cond func(kd *v1alpha1.KubernetesDiscovery) bool, msg string, args ...interface{}) {
	f.t.Helper()
	var desc strings.Builder
	args = append([]interface{}{&desc}, args...)
	msg = "[%s] " + msg
	key := KeyForManifest(mn)
	require.Eventuallyf(f.t, func() bool {
		desc.Reset()
		state := f.store.RLockState()
		defer f.store.RUnlockState()
		storeKD := state.KubernetesDiscoveries[key]
		if storeKD != nil {
			// avoid tests unintentionally modifying state
			storeKD = storeKD.DeepCopy()
		}
		storeOK := cond(storeKD)
		if !storeOK {
			desc.WriteString("Store")
			return false
		}

		var apiKD v1alpha1.KubernetesDiscovery
		err := f.cli.Get(f.ctx, key, &apiKD)
		if apierrors.IsNotFound(err) {
			return cond(nil)
		} else if err != nil {
			desc.WriteString(fmt.Sprintf("API: %v", err))
			return false
		}
		return cond(&apiKD)
	}, stdTimeout, 20*time.Millisecond, msg, args...)
}

func (f *msFixture) requireWatchRefs(mn model.ManifestName, watchRefs ...v1alpha1.KubernetesWatchRef) {
	f.t.Helper()
	var desc strings.Builder
	f.requireDiscoveryState(mn, func(kd *v1alpha1.KubernetesDiscovery) bool {
		desc.Reset()
		if kd == nil {
			desc.WriteString("no spec exists in store")
			return false
		}
		desc.WriteString("\n")
		desc.WriteString(cmp.Diff(watchRefs, kd.Spec.Watches))
		if len(kd.Spec.Watches) != len(watchRefs) {
			return false
		}
		for i, expectedRef := range watchRefs {
			actualRef := kd.Spec.Watches[i]
			if !equality.Semantic.DeepEqual(expectedRef, actualRef) {
				return false
			}
		}
		return true
	}, "KubernetesDiscovery for manifest[%s] does not have expected watch refs: %s", mn, &desc)
}

func (f *msFixture) requireExtraSelectors(mn model.ManifestName, labelSets ...labels.Set) {
	f.t.Helper()
	var desc strings.Builder

	var expectedSelectors []metav1.LabelSelector
	for _, ls := range labelSets {
		expectedSelectors = append(expectedSelectors, *metav1.SetAsLabelSelector(ls))
	}

	f.requireDiscoveryState(mn, func(kd *v1alpha1.KubernetesDiscovery) bool {
		desc.Reset()
		if kd == nil {
			desc.WriteString("no spec exists in apiserver")
			return false
		}
		if equality.Semantic.DeepEqual(expectedSelectors, kd.Spec.ExtraSelectors) {
			return true
		}
		desc.WriteString("\n")
		desc.WriteString(cmp.Diff(expectedSelectors, kd.Spec.ExtraSelectors))
		return false
	}, "KubernetesDiscovery for manifest[%s] does not have expected extra selectors: %s", mn, &desc)
}

func watchRef(namespace k8s.Namespace, uid types.UID, name string) v1alpha1.KubernetesWatchRef {
	return v1alpha1.KubernetesWatchRef{
		Namespace: namespace.String(),
		UID:       string(uid),
		Name:      name,
	}
}

// changeSubscriber helps ensure that a ChangeSummary was populated with the affected KubernetesDiscovery object key.
//
// Other subscribers rely on this being properly populated, so this ensures that the actions dispatched by
// ManifestSubscriber are properly populating it.
type changeSubscriber struct {
	t       testing.TB
	mu      sync.Mutex
	changes store.ChangeSet
}

func newChangeSubscriber(t testing.TB) *changeSubscriber {
	return &changeSubscriber{
		t:       t,
		changes: store.NewChangeSet(),
	}
}

func (c *changeSubscriber) waitForChangeAndReset(key types.NamespacedName) {
	c.t.Helper()
	require.Eventuallyf(c.t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		changed := c.changes.Changes[key]
		if changed {
			c.changes = store.NewChangeSet()
			return true
		}
		return false
	}, stdTimeout, 20*time.Millisecond, "Change for key[%s] was never seen", key.String())
}

func (c *changeSubscriber) OnChange(_ context.Context, _ store.RStore, summary store.ChangeSummary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range summary.KubernetesDiscoveries.Changes {
		c.changes.Add(key)
	}
}
