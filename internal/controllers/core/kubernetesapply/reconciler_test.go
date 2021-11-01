package kubernetesapply

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestImageIndexing(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ImageMaps: []string{"image-a", "image-c"},
		},
	}
	f.Create(&ka)

	// Verify we can index one image map.
	reqs := f.r.indexer.Enqueue(&v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-a"}})
	assert.ElementsMatch(t, []reconcile.Request{
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "a"}},
	}, reqs)

	kb := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "b",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ImageMaps: []string{"image-b", "image-c"},
		},
	}
	f.Create(&kb)

	// Verify we can index one image map to two applies.
	reqs = f.r.indexer.Enqueue(&v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-c"}})
	assert.ElementsMatch(t, []reconcile.Request{
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "a"}},
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "b"}},
	}, reqs)

	ka.Spec.ImageMaps = []string{"image-a"}
	f.Update(&ka)

	// Verify we can remove an image map.
	reqs = f.r.indexer.Enqueue(&v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-c"}})
	assert.ElementsMatch(t, []reconcile.Request{
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "b"}},
	}, reqs)
}

func TestBasicApplyYAML(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: testyaml.SanchoYAML,
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Contains(f.T(), ka.Status.ResultYAML, "name: sancho")
	assert.Contains(f.T(), ka.Status.ResultYAML, "uid:")

	// Make sure that re-reconciling doesn't re-apply the YAML"
	f.kClient.Yaml = ""
	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Equal(f.T(), f.kClient.Yaml, "")
}

func TestBasicApplyCmd(t *testing.T) {
	f := newFixture(t)

	entities, err := k8s.ParseYAMLFromString(testyaml.SanchoYAML)
	require.NoError(t, err, "Could not parse SanchoYAML")
	for i := range entities {
		entities[i].SetUID(uuid.New().String())
	}
	yamlOut, err := k8s.SerializeSpecYAML(entities)
	require.NoError(t, err, "Failed to re-serialize SanchoYAML")

	f.execer.RegisterCommand("custom-apply-cmd", 0, yamlOut, "")

	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			Cmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-apply-cmd"}},
		},
	}
	f.Create(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Empty(t, ka.Status.Error)
	assert.NotZero(t, ka.Status.LastApplyTime)
	assert.Equal(t, yamlOut, ka.Status.ResultYAML)

	// verify that a re-reconcile does NOT re-invoke the command
	f.execer.RegisterCommandError("custom-apply-cmd", errors.New("this should not get invoked"))
	f.MustReconcile(types.NamespacedName{Name: "a"})
	lastApplyTime := ka.Status.LastApplyTime
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Empty(t, ka.Status.Error)
	timecmp.AssertTimeEqual(t, lastApplyTime, ka.Status.LastApplyTime)
}

func TestBasicApplyCmd_ExecError(t *testing.T) {
	f := newFixture(t)

	f.execer.RegisterCommandError("custom-apply-cmd", errors.New("could not start process"))

	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			Cmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-apply-cmd"}},
		},
	}
	f.Create(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	assert.Equal(t, "apply command failed: could not start process", ka.Status.Error)
}

func TestBasicApplyCmd_NonZeroExitCode(t *testing.T) {
	f := newFixture(t)

	f.execer.RegisterCommand("custom-apply-cmd", 77, "whoops", "oh no")

	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "a",
			Annotations: map[string]string{v1alpha1.AnnotationManifest: "foo"},
		},
		Spec: v1alpha1.KubernetesApplySpec{
			Cmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-apply-cmd"}},
		},
	}
	f.Create(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	if assert.Equal(t, "apply command exited with status 77\nstdout:\nwhoops\n", ka.Status.Error) {
		logAction := f.st.WaitForAction(t, reflect.TypeOf(store.LogAction{}))
		assert.Equal(t, `manifest: foo, spanID: KubernetesApply-a, msg: "oh no"`, logAction.(store.LogAction).String())
	}
}

func TestBasicApplyCmd_MalformedYAML(t *testing.T) {
	f := newFixture(t)

	f.execer.RegisterCommand("custom-apply-cmd", 0, "this is not yaml", "")

	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			Cmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-apply-cmd"}},
		},
	}
	f.Create(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	if assert.Contains(t, ka.Status.Error, "apply command returned malformed YAML") {
		assert.Contains(t, ka.Status.Error, "stdout:\nthis is not yaml\n")
	}
}

func TestGarbageCollectAll(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: testyaml.SanchoYAML,
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")

	f.Delete(&ka)
	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.DeletedYaml, "name: sancho")
}

func TestGarbageCollectPartial(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: fmt.Sprintf("%s\n---\n%s\n", testyaml.SanchoYAML, testyaml.PodDisruptionBudgetYAML),
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")
	assert.Contains(f.T(), f.kClient.Yaml, "name: infra-kafka-zookeeper")

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	ka.Spec.YAML = testyaml.SanchoYAML
	f.Update(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")
	assert.NotContains(f.T(), f.kClient.Yaml, "name: infra-kafka-zookeeper")
	assert.Contains(f.T(), f.kClient.DeletedYaml, "name: infra-kafka-zookeeper")
}

func TestGarbageCollectAfterErrorDuringApply(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: fmt.Sprintf("%s\n---\n%s\n", testyaml.SanchoYAML, testyaml.PodDisruptionBudgetYAML),
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")
	assert.Contains(f.T(), f.kClient.Yaml, "name: infra-kafka-zookeeper")

	f.kClient.UpsertError = errors.New("oh no")

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	ka.Spec.YAML = testyaml.SanchoYAML
	f.Update(&ka)

	// because the apply (upsert) returned an error, no GC should have happened yet
	f.MustReconcile(types.NamespacedName{Name: "a"})
	if assert.Empty(t, f.kClient.DeletedYaml) {
		assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")
		assert.Contains(f.T(), f.kClient.Yaml, "name: infra-kafka-zookeeper")
	}
}

func TestRestartOn(t *testing.T) {
	f := newFixture(t)

	f.Create(&v1alpha1.FileWatch{
		ObjectMeta: metav1.ObjectMeta{Name: "fw"},
		Spec:       v1alpha1.FileWatchSpec{WatchedPaths: []string{"/fake/dir"}},
	})

	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: testyaml.SanchoYAML,
			RestartOn: &v1alpha1.RestartOnSpec{
				FileWatches: []string{"fw"},
			},
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Contains(f.T(), ka.Status.ResultYAML, "name: sancho")
	assert.Contains(f.T(), ka.Status.ResultYAML, "uid:")
	lastApply := ka.Status.LastApplyTime

	// Make sure that re-reconciling w/o changes doesn't re-apply the YAML
	f.kClient.Yaml = ""
	f.MustReconcile(types.NamespacedName{Name: "a"})
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Equal(f.T(), f.kClient.Yaml, "")
	timecmp.AssertTimeEqual(t, lastApply, ka.Status.LastApplyTime)

	// Fake a FileWatch event - now re-reconciling should re-apply the YAML
	var fw v1alpha1.FileWatch
	f.MustGet(types.NamespacedName{Name: "fw"}, &fw)
	ts := apis.NowMicro()
	fw.Status.LastEventTime = ts
	fw.Status.FileEvents = append(fw.Status.FileEvents, v1alpha1.FileEvent{
		Time:      ts,
		SeenFiles: []string{"/fake/dir/file"},
	})
	f.UpdateStatus(&fw)

	f.kClient.Yaml = ""
	f.MustReconcile(types.NamespacedName{Name: "a"})
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")
	assert.Truef(t, ka.Status.LastApplyTime.After(lastApply.Time),
		"Last apply time %s should have been after previous apply time %s",
		ka.Status.LastApplyTime.Format(time.RFC3339Nano),
		lastApply.Format(time.RFC3339Nano))
	lastApply = ka.Status.LastApplyTime

	// One last time - make sure that re-reconciling w/o changes doesn't re-apply the YAML
	f.kClient.Yaml = ""
	f.MustReconcile(types.NamespacedName{Name: "a"})
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Equal(f.T(), f.kClient.Yaml, "")
	timecmp.AssertTimeEqual(f.T(), lastApply, ka.Status.LastApplyTime)
}

type fixture struct {
	*fake.ControllerFixture
	r       *Reconciler
	kClient *k8s.FakeK8sClient
	execer  *localexec.FakeExecer
	st      *store.TestingStore
}

func newFixture(t *testing.T) *fixture {
	kClient := k8s.NewFakeK8sClient(t)
	cfb := fake.NewControllerFixtureBuilder(t)
	st := store.NewTestingStore()
	dockerClient := docker.NewFakeClient()
	kubeContext := k8s.KubeContext("kind-kind")

	// Make the fake ImageExists always return true, which is the behavior we want
	// when testing the reconciler
	dockerClient.ImageAlwaysExists = true

	execer := localexec.NewFakeExecer(t)

	db := build.NewDockerImageBuilder(dockerClient, dockerfile.Labels{})
	r := NewReconciler(cfb.Client, kClient, v1alpha1.NewScheme(), db, kubeContext, st, "default", execer)

	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
		kClient:           kClient,
		execer:            execer,
		st:                st,
	}
}
