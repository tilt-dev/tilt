package kubernetesapply

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/testutils/configmap"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Test constants
const timeout = time.Second * 10
const interval = 5 * time.Millisecond

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
	ctx := context.Background()
	reqs := f.r.indexer.Enqueue(ctx, &v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-a"}})
	assert.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: "a"}},
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
	reqs = f.r.indexer.Enqueue(ctx, &v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-c"}})
	assert.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: "a"}},
		{NamespacedName: types.NamespacedName{Name: "b"}},
	}, reqs)

	// Get the latest ka, since resource version numbers
	// may have changed since its creation and mismatched
	// versions will throw an error on update
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	ka.Spec.ImageMaps = []string{"image-a"}
	f.Update(&ka)

	// Verify we can remove an image map.
	reqs = f.r.indexer.Enqueue(ctx, &v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-c"}})
	assert.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: "b"}},
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

	assert.Contains(t, f.Stdout(),
		"Objects applied to cluster:\n       → sancho:deployment\n",
		"Log output did not include applied objects")

	// Make sure that re-reconciling doesn't re-apply the YAML"
	f.kClient.Yaml = ""
	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Equal(f.T(), f.kClient.Yaml, "")
}

func TestBasicApplyCmd(t *testing.T) {
	f := newFixture(t)

	applyCmd, yamlOut := f.createApplyCmd("custom-apply-cmd", testyaml.SanchoYAML)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ApplyCmd:  &applyCmd,
			DeleteCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-delete-cmd"}},
		},
	}
	f.Create(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Empty(t, ka.Status.Error)
	assert.NotZero(t, ka.Status.LastApplyTime)
	assert.Equal(t, yamlOut, ka.Status.ResultYAML)

	assert.Contains(t, f.Stdout(),
		"Running cmd: custom-apply-cmd\n     Objects applied to cluster:\n       → sancho:deployment\n",
		"Log output did not include applied objects")
	assert.Equal(t, 1, strings.Count(f.Stdout(), "Running cmd"))

	// verify that a re-reconcile does NOT re-invoke the command
	f.execer.RegisterCommandError("custom-apply-cmd", errors.New("this should not get invoked"))
	f.MustReconcile(types.NamespacedName{Name: "a"})
	lastApplyTime := ka.Status.LastApplyTime
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Empty(t, ka.Status.Error)
	timecmp.AssertTimeEqual(t, lastApplyTime, ka.Status.LastApplyTime)

	assert.Len(t, f.execer.Calls(), 1)
}

func TestApplyCmdWithImages(t *testing.T) {
	f := newFixture(t)

	f.Create(&v1alpha1.ImageMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-a",
		},
		Status: v1alpha1.ImageMapStatus{
			Image:            "image-a:my-tag",
			ImageFromCluster: "image-a:my-tag",
		},
	})

	applyCmd, _ := f.createApplyCmd("custom-apply-cmd", testyaml.SanchoYAML)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ImageMaps: []string{"image-a"},
			ApplyCmd:  &applyCmd,
			DeleteCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-delete-cmd"}},
		},
	}
	f.Create(&ka)

	if assert.Len(t, f.execer.Calls(), 1) {
		call := f.execer.Calls()[0]
		assert.Equal(t, []string{
			"TILT_IMAGE_MAP_0=image-a",
			"TILT_IMAGE_0=image-a:my-tag",
		}, call.Cmd.Env)
	}
}

func TestApplyCmdWithKubeconfig(t *testing.T) {
	f := newFixture(t)

	f.Create(&v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-cluster",
		},
		Status: v1alpha1.ClusterStatus{
			Connection: &v1alpha1.ClusterConnectionStatus{
				Kubernetes: &v1alpha1.KubernetesClusterConnectionStatus{
					ConfigPath: "/path/to/my/kubeconfig",
				},
			},
		},
	})

	applyCmd, _ := f.createApplyCmd("custom-apply-cmd", testyaml.SanchoYAML)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			Cluster:   "default-cluster",
			ApplyCmd:  &applyCmd,
			DeleteCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-delete-cmd"}},
		},
	}
	f.Create(&ka)

	if assert.Len(t, f.execer.Calls(), 1) {
		call := f.execer.Calls()[0]
		assert.Equal(t, []string{"custom-apply-cmd"}, call.Cmd.Argv)
		assert.Equal(t, []string{
			"KUBECONFIG=/path/to/my/kubeconfig",
		}, call.Cmd.Env)
	}

	f.Delete(&ka)

	if assert.Len(t, f.execer.Calls(), 2) {
		call := f.execer.Calls()[1]
		assert.Equal(t, []string{"custom-delete-cmd"}, call.Cmd.Argv)
		assert.Equal(t, []string{
			"KUBECONFIG=/path/to/my/kubeconfig",
		}, call.Cmd.Env)
	}

}

func TestBasicApplyCmd_ExecError(t *testing.T) {
	f := newFixture(t)

	f.execer.RegisterCommandError("custom-apply-cmd", errors.New("could not start process"))

	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ApplyCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-apply-cmd"}},
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
			ApplyCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-apply-cmd"}},
		},
	}
	f.Create(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	if assert.Equal(t, "apply command exited with status 77\nstdout:\nwhoops\n\n", ka.Status.Error) {
		assert.Contains(t, f.Stdout(), `oh no`)
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
			ApplyCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-apply-cmd"}},
		},
	}
	f.Create(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	if assert.Contains(t, ka.Status.Error, "apply command returned malformed YAML") {
		assert.Contains(t, ka.Status.Error, "stdout:\nthis is not yaml\n")
	}
}

func TestBasicApplyYAML_JobComplete(t *testing.T) {
	f := newFixture(t)

	jobYAML := testyaml.JobYAML
	entities, err := k8s.ParseYAMLFromString(jobYAML)
	require.NoError(t, err, "Invalid JobYAML")
	require.Len(t, entities, 1, "Expected exactly 1 Job entity")
	require.IsType(t, entities[0].Obj, &batchv1.Job{}, "Expected exactly 1 Job entity")
	job := entities[0].Obj.(*batchv1.Job)
	job.SetUID(uuid.NewUUID())
	job.Status = batchv1.JobStatus{
		Conditions: []batchv1.JobCondition{
			{
				Type:   batchv1.JobComplete,
				Status: v1.ConditionTrue,
			},
		},
	}
	f.kClient.UpsertResult = entities

	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: testyaml.JobYAML,
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	expected := []metav1.Condition{
		{
			Type:   "JobComplete",
			Status: metav1.ConditionTrue,
		},
	}

	assert.Equal(f.T(), ka.Status.Conditions, expected,
		"KubernetesApply status should reflect Job completion")

	// Make sure that re-reconciling doesn't clear the conditions
	f.MustReconcile(types.NamespacedName{Name: "a"})
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Equal(f.T(), ka.Status.Conditions, expected,
		"KubernetesApply status should reflect Job completion")
}

func TestGarbageCollectAllOnDelete_YAML(t *testing.T) {
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

func TestGarbageCollectAllOnDelete_Cmd(t *testing.T) {
	f := newFixture(t)

	applyCmd, yamlOut := f.createApplyCmd("custom-apply-cmd", testyaml.SanchoYAML)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ApplyCmd:  &applyCmd,
			DeleteCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-delete-cmd"}},
		},
	}
	f.Create(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Equal(f.T(), yamlOut, ka.Status.ResultYAML)

	f.Delete(&ka)
	assert.False(t, f.Get(types.NamespacedName{Name: "a"}, &ka), "Object was not deleted")

	calls := f.execer.Calls()
	if assert.Len(t, calls, 2, "Expected 2 calls (1x apply + 1x delete)") {
		assert.Equal(t, []string{"custom-delete-cmd"}, calls[1].Cmd.Argv)
	}
}

func TestGarbageCollectAllOnDisable(t *testing.T) {
	f := newFixture(t)

	applyCmd, yamlOut := f.createApplyCmd("custom-apply-cmd", testyaml.SanchoYAML)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ApplyCmd:  &applyCmd,
			DeleteCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-delete-cmd"}},
			DisableSource: &v1alpha1.DisableSource{
				ConfigMap: &v1alpha1.ConfigMapDisableSource{
					Name: "test-disable",
					Key:  "isDisabled",
				},
			},
		},
	}
	f.Create(&ka)

	f.setDisabled(ka.GetObjectMeta().Name, false)
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Equal(f.T(), yamlOut, ka.Status.ResultYAML)

	f.setDisabled(ka.GetObjectMeta().Name, true)

	calls := f.execer.Calls()
	if assert.Len(t, calls, 2, "Expected 2 calls (1x apply + 1x delete)") {
		assert.Equal(t, []string{"custom-apply-cmd"}, calls[0].Cmd.Argv)
		assert.Equal(t, []string{"custom-delete-cmd"}, calls[1].Cmd.Argv)
	}

	f.setDisabled(ka.GetObjectMeta().Name, false)
	calls = f.execer.Calls()
	if assert.Len(t, calls, 3, "Expected 3 calls (2x apply + 1x delete)") {
		assert.Equal(t, []string{"custom-apply-cmd"}, calls[2].Cmd.Argv)
	}

	// Confirm the k8s client never deletes resources directly.
	assert.Equal(t, "", f.kClient.DeletedYaml)
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

	assert.Contains(f.T(), f.Stdout(), "Tried to apply objects")
}

func TestGarbageCollect_DeleteCmdNotInvokedOnChange(t *testing.T) {
	f := newFixture(t)

	applyCmd, yamlOut := f.createApplyCmd("custom-apply-1", testyaml.SanchoYAML)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ApplyCmd:  &applyCmd,
			DeleteCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-delete-cmd"}},
		},
	}
	f.Create(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Equal(f.T(), yamlOut, ka.Status.ResultYAML)

	applyCmd, yamlOut = f.createApplyCmd("custom-apply-2", testyaml.JobYAML)
	ka.Spec.ApplyCmd = &applyCmd
	f.Update(&ka)

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Equal(f.T(), yamlOut, ka.Status.ResultYAML)

	calls := f.execer.Calls()
	if assert.Len(t, calls, 2, "Expected 2x calls (both to apply)") {
		for i := range calls {
			assert.Equal(t, []string{fmt.Sprintf("custom-apply-%d", i+1)}, calls[i].Cmd.Argv)
		}
	}

	if assert.NoError(t, f.kClient.DeleteError,
		"Delete should not have been invoked (so no error should have occurred)") {
		assert.Empty(t, f.kClient.DeletedYaml,
			"Delete should not have been invoked (so no YAML should have been deleted)")
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

func TestIgnoreManagedObjects(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
			Annotations: map[string]string{
				v1alpha1.AnnotationManagedBy: "buildcontrol",
			},
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: testyaml.SanchoYAML,
		},
	}
	f.Create(&ka)

	nn := types.NamespacedName{Name: "a"}
	f.MustReconcile(nn)
	assert.Empty(f.T(), f.kClient.Yaml)

	// no apply should happen since the object is managed by the engine
	f.MustGet(nn, &ka)
	assert.Empty(f.T(), ka.Status.ResultYAML)
	assert.Zero(f.T(), ka.Status.LastApplyTime)

	result := f.r.ForceApply(f.Context(), nn, ka.Spec, nil, nil)
	assert.Contains(f.T(), result.ResultYAML, "sancho")
	assert.True(f.T(), !result.LastApplyTime.IsZero())
	assert.True(f.T(), !result.LastApplyStartTime.IsZero())
	assert.Equal(f.T(), result.Error, "")

	// ForceApply must NOT update the apiserver.
	f.MustGet(nn, &ka)
	assert.Empty(f.T(), ka.Status.ResultYAML)
	assert.Zero(f.T(), ka.Status.LastApplyTime)

	f.MustReconcile(nn)
	f.MustGet(nn, &ka)
	assert.Equal(f.T(), result, ka.Status)
}

func TestForceDelete(t *testing.T) {
	f := newFixture(t)
	nn := types.NamespacedName{Name: "a"}
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
			Annotations: map[string]string{
				v1alpha1.AnnotationManagedBy: "buildcontrol",
			},
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: testyaml.SanchoYAML,
		},
	}
	f.Create(&ka)

	err := f.r.ForceDelete(f.Context(), nn, ka.Spec, nil, "testing")
	assert.Nil(f.T(), err)
	assert.Contains(f.T(), f.kClient.DeletedYaml, "sancho")
}

func TestForceDeleteWithCmd(t *testing.T) {
	f := newFixture(t)

	nn := types.NamespacedName{Name: "a"}
	applyCmd, _ := f.createApplyCmd("custom-apply-cmd", testyaml.SanchoYAML)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
			Annotations: map[string]string{
				v1alpha1.AnnotationManagedBy: "buildcontrol",
			},
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ApplyCmd:  &applyCmd,
			DeleteCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-delete-cmd"}},
		},
	}
	f.Create(&ka)

	err := f.r.ForceDelete(f.Context(), nn, ka.Spec, nil, "testing")
	assert.Nil(f.T(), err)

	calls := f.execer.Calls()
	if assert.Len(t, calls, 1, "Expected 1x delete calls") {
		assert.Equal(t, []string{"custom-delete-cmd"}, calls[0].Cmd.Argv)
	}
}

func TestDisableByConfigmap(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: testyaml.SanchoYAML,
			DisableSource: &v1alpha1.DisableSource{
				ConfigMap: &v1alpha1.ConfigMapDisableSource{
					Name: "test-disable",
					Key:  "isDisabled",
				},
			},
		},
	}
	f.Create(&ka)

	f.setDisabled(ka.GetObjectMeta().Name, false)

	f.setDisabled(ka.GetObjectMeta().Name, true)

	f.setDisabled(ka.GetObjectMeta().Name, false)
}

func (f *fixture) requireKaMatchesInApi(name string, matcher func(ka *v1alpha1.KubernetesApply) bool) *v1alpha1.KubernetesApply {
	ka := v1alpha1.KubernetesApply{}

	require.Eventually(f.T(), func() bool {
		f.MustGet(types.NamespacedName{Name: name}, &ka)
		return matcher(&ka)
	}, timeout, interval)

	return &ka
}

func (f *fixture) setDisabled(name string, isDisabled bool) {
	ka := v1alpha1.KubernetesApply{}
	f.MustGet(types.NamespacedName{Name: name}, &ka)

	require.NotNil(f.T(), ka.Spec.DisableSource)
	require.NotNil(f.T(), ka.Spec.DisableSource.ConfigMap)

	ds := ka.Spec.DisableSource.ConfigMap
	err := configmap.UpsertDisableConfigMap(f.Context(), f.Client, ds.Name, ds.Key, isDisabled)
	require.NoError(f.T(), err)

	_, err = f.Reconcile(types.NamespacedName{Name: name})
	require.NoError(f.T(), err)

	f.requireKaMatchesInApi(name, func(ka *v1alpha1.KubernetesApply) bool {
		return ka.Status.DisableStatus != nil && ka.Status.DisableStatus.Disabled == isDisabled
	})

	// the KA reconciler only creates a KD if the yaml is already in the KA's status,
	// which means we need a second call to Reconcile to get the KD
	_, err = f.Reconcile(types.NamespacedName{Name: name})
	require.NoError(f.T(), err)

	kd := v1alpha1.KubernetesDiscovery{}
	kdExists := f.Get(types.NamespacedName{Name: name}, &kd)

	if isDisabled {
		require.False(f.T(), kdExists)

		if ka.Spec.DeleteCmd == nil {
			require.Contains(f.T(), f.kClient.DeletedYaml, "name: sancho")
		} else {
			// Must run the delete cmd instead of deleting resources with our k8s client.
			require.Equal(f.T(), f.kClient.DeletedYaml, "")
		}

		// Reset the deletedYaml so it doesn't interfere with other tests
		f.kClient.DeletedYaml = ""
	} else {
		require.True(f.T(), kdExists)
	}
}

type fixture struct {
	*fake.ControllerFixture
	r       *Reconciler
	kClient *k8s.FakeK8sClient
	execer  *localexec.FakeExecer
}

func newFixture(t *testing.T) *fixture {
	kClient := k8s.NewFakeK8sClient(t)
	cfb := fake.NewControllerFixtureBuilder(t)
	dockerClient := docker.NewFakeClient()

	// Make the fake ImageExists always return true, which is the behavior we want
	// when testing the reconciler
	dockerClient.ImageAlwaysExists = true

	execer := localexec.NewFakeExecer(t)

	r := NewReconciler(cfb.Client, kClient, v1alpha1.NewScheme(), cfb.Store, execer)

	f := &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
		kClient:           kClient,
		execer:            execer,
	}
	f.Create(&v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		Status: v1alpha1.ClusterStatus{
			Connection: &v1alpha1.ClusterConnectionStatus{
				Kubernetes: &v1alpha1.KubernetesClusterConnectionStatus{
					Context: "default",
				},
			},
		},
	})

	return f
}

// createApplyCmd creates a KubernetesApplyCmd that use the passed YAML to generate simulated stdout via the FakeExecer.
func (f *fixture) createApplyCmd(name string, yaml string) (v1alpha1.KubernetesApplyCmd, string) {
	f.T().Helper()

	require.NotEmpty(f.T(), yaml, "applyCmd YAML cannot be blank")

	entities, err := k8s.ParseYAMLFromString(yaml)
	require.NoErrorf(f.T(), err, "Could not parse YAML: %s", yaml)
	for i := range entities {
		entities[i].SetUID(string(uuid.NewUUID()))
	}
	yamlOut, err := k8s.SerializeSpecYAML(entities)
	require.NoErrorf(f.T(), err, "Failed to re-serialize YAML for entities: %s", spew.Sdump(entities))

	f.execer.RegisterCommand(name, 0, yamlOut, "")
	return v1alpha1.KubernetesApplyCmd{
		Args: []string{name},
	}, yamlOut
}
