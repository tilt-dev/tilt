package buildcontrol

import (
	"archive/tar"
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/yaml"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/wmclient/pkg/dirs"
)

func TestDeployTwinImages(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	sancho := NewSanchoDockerBuildManifest(f)
	newK8sTarget := k8s.MustTarget("sancho", yaml.ConcatYAML(SanchoYAML, SanchoTwinYAML)).
		WithImageDependencies(sancho.K8sTarget().ImageMaps)
	manifest := sancho.WithDeployTarget(newK8sTarget)
	result, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	id := manifest.ImageTargetAt(0).ID()
	expectedImage := "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	image := store.ClusterImageRefFromBuildResult(result[id])
	assert.Equal(t, expectedImage, image)
	assert.Equalf(t, 2, strings.Count(f.k8s.Yaml, expectedImage),
		"Expected image to update twice in YAML: %s", f.k8s.Yaml)
}

func TestForceUpdate(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	m := NewSanchoDockerBuildManifest(f)

	iTargetID1 := m.ImageTargets[0].ID()
	stateSet := store.BuildStateSet{
		iTargetID1: store.BuildState{FullBuildTriggered: true},
	}
	_, err := f.BuildAndDeploy(BuildTargets(m), stateSet)
	require.NoError(t, err)

	// A force rebuild should delete the old resources.
	assert.Equal(t, 1, strings.Count(f.k8s.DeletedYaml, "Deployment"))
}

func TestForceUpdateDoesNotDeleteNamespace(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML + `
---
apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: my-namespace
`).
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		Build()

	iTargetID1 := m.ImageTargets[0].ID()
	stateSet := store.BuildStateSet{
		iTargetID1: store.BuildState{FullBuildTriggered: true},
	}
	_, err := f.BuildAndDeploy(BuildTargets(m), stateSet)
	require.NoError(t, err)

	// A force rebuild should delete the ConfigMap but not the Namespace.
	assert.Equal(t, 1, strings.Count(f.k8s.DeletedYaml, "kind: ConfigMap"))
	assert.Equal(t, 0, strings.Count(f.k8s.DeletedYaml, "kind: Namespace"))
}

func TestDeleteShouldHappenInReverseOrder(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	m := newK8sMultiEntityManifest("sancho")

	err := f.ibd.delete(f.ctx, m.K8sTarget())
	require.NoError(t, err)

	assert.Regexp(t, "(?s)name: sancho-deployment.*name: sancho-pvc", f.k8s.DeletedYaml) // pvc comes after deployment
}

func TestDeployPodWithMultipleImages(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	iTarget1 := NewSanchoDockerBuildImageTarget(f)
	iTarget2 := NewSanchoSidecarDockerBuildImageTarget(f)
	kTarget := k8s.MustTarget("sancho", testyaml.SanchoSidecarYAML).
		WithImageDependencies([]string{iTarget1.ImageMapName(), iTarget2.ImageMapName()})
	targets := []model.TargetSpec{iTarget1, iTarget2, kTarget}

	result, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)

	expectedSanchoRef := "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	image := store.ClusterImageRefFromBuildResult(result[iTarget1.ID()])
	assert.Equal(t, expectedSanchoRef, image)
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSanchoRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)

	expectedSidecarRef := "gcr.io/some-project-162817/sancho-sidecar:tilt-11cd0b38bc3ceb95"
	image = store.ClusterImageRefFromBuildResult(result[iTarget2.ID()])
	assert.Equal(t, expectedSidecarRef, image)
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSidecarRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)
}

func TestDeployPodWithMultipleLiveUpdateImages(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	iTarget1 := NewSanchoLiveUpdateImageTarget(f)
	iTarget2 := NewSanchoSidecarLiveUpdateImageTarget(f)

	kTarget := k8s.MustTarget("sancho", testyaml.SanchoSidecarYAML).
		WithImageDependencies([]string{iTarget1.ImageMapName(), iTarget2.ImageMapName()})
	targets := []model.TargetSpec{iTarget1, iTarget2, kTarget}

	result, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)

	expectedSanchoRef := "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	image := store.ClusterImageRefFromBuildResult(result[iTarget1.ID()])
	assert.Equal(t, expectedSanchoRef, image)
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSanchoRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)

	expectedSidecarRef := "gcr.io/some-project-162817/sancho-sidecar:tilt-11cd0b38bc3ceb95"
	image = store.ClusterImageRefFromBuildResult(result[iTarget2.ID()])
	assert.Equal(t, expectedSidecarRef, image)
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSidecarRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)
}

func TestNoImageTargets(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	targName := "some-k8s-manifest"
	specs := []model.TargetSpec{
		k8s.MustTarget(model.TargetName(targName), testyaml.LonelyPodYAML),
	}

	_, err := f.BuildAndDeploy(specs, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount, "expect no docker builds")
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, "image: gcr.io/windmill-public-containers/lonely-pod"),
		"Expected lonely-pod image to appear once in YAML: %s", f.k8s.Yaml)

	expectedLabelStr := fmt.Sprintf("%s: %s", k8s.ManagedByLabel, k8s.ManagedByValue)
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedLabelStr),
		"Expected \"%s\" label to appear once in YAML: %s", expectedLabelStr, f.k8s.Yaml)

	// If we're not making updates in response to an image change, it's OK to
	// leave the existing image pull policy.
	assert.Contains(t, f.k8s.Yaml, "imagePullPolicy: Always")
}

func TestStatefulSetPodManagementPolicy(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	targName := "redis"

	iTarget := NewSanchoDockerBuildImageTarget(f)
	yaml := strings.Replace(
		testyaml.RedisStatefulSetYAML,
		`image: "docker.io/bitnami/redis:4.0.12"`,
		fmt.Sprintf(`image: %q`, f.refs(iTarget).LocalRef().String()), 1)
	kTarget := k8s.MustTarget(model.TargetName(targName), yaml)

	_, err := f.BuildAndDeploy(
		[]model.TargetSpec{kTarget}, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, err)
	assert.NotContains(t, f.k8s.Yaml, "podManagementPolicy: Parallel")

	_, err = f.BuildAndDeploy(
		[]model.TargetSpec{
			iTarget,
			kTarget.WithImageDependencies([]string{iTarget.ImageMapName()}),
		},
		store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, err)
	assert.Contains(t, f.k8s.Yaml, "podManagementPolicy: Parallel")
}

func TestImageIsClean(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	manifest := NewSanchoDockerBuildManifest(f)
	iTargetID1 := manifest.ImageTargets[0].ID()
	result1 := store.NewImageBuildResultSingleRef(iTargetID1, container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"))

	stateSet := store.BuildStateSet{
		iTargetID1: store.NewBuildState(result1, []string{}, nil),
	}
	_, err := f.BuildAndDeploy(BuildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	// Expect no build or push, b/c image is clean (i.e. last build was an image build and
	// no file changes since).
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
}

func TestMultiStageDockerBuild(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	manifest := NewSanchoDockerBuildMultiStageManifest(f)
	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 1, f.docker.PushCount)
	assert.Equal(t, 0, f.kl.loadCount)

	expected := testutils.ExpectedFile{
		Path: "Dockerfile",
		Contents: `
FROM sancho-base:tilt-11cd0b38bc3ceb95
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildContext), expected)
}

func TestMultiStageDockerBuildPreservesSyntaxDirective(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	baseImage := model.MustNewImageTarget(SanchoBaseRef).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM golang:1.10`,
			Context:            f.JoinPath("sancho-base"),
		})

	srcImage := model.MustNewImageTarget(SanchoRef).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `# syntax = docker/dockerfile:experimental

FROM sancho-base
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`,
			Context: f.JoinPath("sancho"),
		}).WithImageMapDeps([]string{baseImage.ImageMapName()})

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTargets(baseImage, srcImage).
		Build()

	_, err := f.BuildAndDeploy(BuildTargets(m), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 1, f.docker.PushCount)
	assert.Equal(t, 0, f.kl.loadCount)

	expected := testutils.ExpectedFile{
		Path: "Dockerfile",
		Contents: `# syntax = docker/dockerfile:experimental

FROM sancho-base:tilt-11cd0b38bc3ceb95
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildContext), expected)
}

func TestMultiStageDockerBuildWithFirstImageDirty(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	manifest := NewSanchoDockerBuildMultiStageManifest(f)
	iTargetID1 := manifest.ImageTargets[0].ID()
	iTargetID2 := manifest.ImageTargets[1].ID()
	result1 := store.NewImageBuildResultSingleRef(iTargetID1, container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"))
	result2 := store.NewImageBuildResultSingleRef(iTargetID2, container.MustParseNamedTagged("sancho:tilt-prebuilt2"))

	newFile := f.WriteFile("sancho-base/message.txt", "message")

	stateSet := store.BuildStateSet{
		iTargetID1: store.NewBuildState(result1, []string{newFile}, nil),
		iTargetID2: store.NewBuildState(result2, nil, nil),
	}
	_, err := f.BuildAndDeploy(BuildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 1, f.docker.PushCount)

	expected := testutils.ExpectedFile{
		Path: "Dockerfile",
		Contents: `
FROM sancho-base:tilt-11cd0b38bc3ceb95
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildContext), expected)
}

func TestMultiStageDockerBuildWithSecondImageDirty(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	manifest := NewSanchoDockerBuildMultiStageManifest(f)
	iTargetID1 := manifest.ImageTargets[0].ID()
	iTargetID2 := manifest.ImageTargets[1].ID()
	result1 := store.NewImageBuildResultSingleRef(iTargetID1, container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"))
	result2 := store.NewImageBuildResultSingleRef(iTargetID2, container.MustParseNamedTagged("sancho:tilt-prebuilt2"))

	newFile := f.WriteFile("sancho/message.txt", "message")

	stateSet := store.BuildStateSet{
		iTargetID1: store.NewBuildState(result1, nil, nil),
		iTargetID2: store.NewBuildState(result2, []string{newFile}, nil),
	}
	_, err := f.BuildAndDeploy(BuildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount)

	expected := testutils.ExpectedFile{
		Path: "Dockerfile",
		Contents: `
FROM sancho-base:tilt-prebuilt1
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildContext), expected)
}

func TestK8sUpsertTimeout(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	timeout := 123 * time.Second

	manifest := NewSanchoDockerBuildManifest(f)
	k8sTarget := manifest.DeployTarget.(model.K8sTarget)
	k8sTarget.Timeout = metav1.Duration{Duration: timeout}
	manifest.DeployTarget = k8sTarget

	_, err := f.BuildAndDeploy(BuildTargets(manifest), nil)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, f.k8s.UpsertTimeout, timeout)
}

func TestKINDLoad(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductKIND)

	manifest := NewSanchoDockerBuildManifest(f)
	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount)
	assert.Equal(t, 1, f.kl.loadCount)
	assert.Equal(t, 0, f.docker.PushCount)
}

func TestDockerPushIfKINDAndClusterRef(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductKIND)
	f.cluster.Spec.DefaultRegistry = &v1alpha1.RegistryHosting{
		Host:                     "localhost:1234",
		HostFromContainerRuntime: "registry:1234",
	}

	manifest := NewSanchoDockerBuildManifest(f)
	iTarg := manifest.ImageTargetAt(0)
	manifest = manifest.WithImageTarget(iTarg)

	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	refs := f.refs(iTarg)

	assert.Equal(t, 1, f.docker.BuildCount, "Docker build count")
	assert.Equal(t, 0, f.kl.loadCount, "KIND load count")
	assert.Equal(t, 1, f.docker.PushCount, "Docker push count")
	assert.Equal(t, refs.LocalRef().String(), container.MustParseNamed(f.docker.PushImage).Name(), "image pushed to Docker as LocalRef")

	yaml := f.k8s.Yaml
	assert.Contains(t, yaml, refs.ClusterRef().String(), "ClusterRef was injected into applied YAML")
	assert.NotContains(t, yaml, refs.LocalRef().String(), "LocalRef was NOT injected into applied YAML")
}

func TestCustomBuildDisablePush(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductKIND)
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build"] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifestWithPushDisabled(f)
	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	assert.NoError(t, err)

	// We didn't try to build or push an image, but we did try to tag it
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 1, f.docker.TagCount)
	assert.Equal(t, 0, f.kl.loadCount)
	assert.Equal(t, 0, f.docker.PushCount)
}

func TestCustomBuildSkipsLocalDocker(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductKIND)
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build"] = types.ImageInspect{ID: string(sha)}

	cb := model.CustomBuild{
		CmdImageSpec: v1alpha1.CmdImageSpec{
			Args:       model.ToHostCmd("exit 0").Argv,
			OutputTag:  "tilt-build",
			OutputMode: v1alpha1.CmdImageOutputRemote,
		},
		Deps: []string{f.JoinPath("app")},
	}

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(model.MustNewImageTarget(SanchoRef).WithBuildDetails(cb)).
		Build()

	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	assert.NoError(t, err)

	// We didn't try to build, tag, or push an image
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.TagCount)
	assert.Equal(t, 0, f.kl.loadCount)
	assert.Equal(t, 0, f.docker.PushCount)
}

func TestBuildAndDeployUsesCorrectRef(t *testing.T) {
	expectedImages := []string{"foo.com/gcr.io_some-project-162817_sancho"}
	expectedImagesClusterRef := []string{"registry:1234/gcr.io_some-project-162817_sancho"}
	tests := []struct {
		name           string
		manifest       func(f Fixture) model.Manifest
		withClusterRef bool // if true, clusterRef != localRef, i.e. ref of the built docker image != ref injected into YAML
		expectBuilt    []string
		expectDeployed []string
	}{
		{"docker build", NewSanchoDockerBuildManifest, false, expectedImages, expectedImages},
		{"docker build + distinct clusterRef", NewSanchoDockerBuildManifest, true, expectedImages, expectedImagesClusterRef},
		{"custom build", NewSanchoCustomBuildManifest, false, expectedImages, expectedImages},
		{"custom build + distinct clusterRef", NewSanchoCustomBuildManifest, true, expectedImages, expectedImagesClusterRef},
		{"live multi stage", NewSanchoLiveUpdateMultiStageManifest, false, append(expectedImages, "foo.com/sancho-base"), expectedImages},
		{"live multi stage + distinct clusterRef", NewSanchoLiveUpdateMultiStageManifest, true, append(expectedImages, "foo.com/sancho-base"), expectedImagesClusterRef},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newIBDFixture(t, clusterid.ProductGKE)
			f.cluster.Spec.DefaultRegistry = &v1alpha1.RegistryHosting{Host: "foo.com"}
			if test.withClusterRef {
				f.cluster.Spec.DefaultRegistry.HostFromContainerRuntime = "registry:1234"
			}

			if strings.Contains(test.name, "custom build") {
				sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
				f.docker.Images["foo.com/gcr.io_some-project-162817_sancho:tilt-build-1546304461"] = types.ImageInspect{ID: string(sha)}
			}

			manifest := test.manifest(f)
			result, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
			if err != nil {
				t.Fatal(err)
			}

			var observedImages []string
			for i := range manifest.ImageTargets {
				id := manifest.ImageTargets[i].ID()
				image := store.LocalImageRefFromBuildResult(result[id])
				imageRef := container.MustParseNamedTagged(image)
				observedImages = append(observedImages, imageRef.Name())
			}

			assert.ElementsMatch(t, test.expectBuilt, observedImages)

			for _, expected := range test.expectDeployed {
				assert.Contains(t, f.k8s.Yaml, expected)
			}
		})
	}
}

func TestDeployInjectImageEnvVar(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	manifest := NewSanchoManifestWithImageInEnvVar(f)
	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	entities, err := k8s.ParseYAMLFromString(f.k8s.Yaml)
	if err != nil {
		t.Fatal(err)
	}

	if !assert.Equal(t, 1, len(entities)) {
		return
	}

	d := entities[0].Obj.(*v1.Deployment)
	if !assert.Equal(t, 1, len(d.Spec.Template.Spec.Containers)) {
		return
	}

	c := d.Spec.Template.Spec.Containers[0]
	// container image always gets injected
	assert.Equal(t, "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95", c.Image)
	expectedEnv := []corev1.EnvVar{
		// sancho2 gets injected here because it sets match_in_env_vars in docker_build
		{Name: "foo", Value: "gcr.io/some-project-162817/sancho2:tilt-11cd0b38bc3ceb95"},
		// sancho does not because it doesn't
		{Name: "bar", Value: "gcr.io/some-project-162817/sancho"},
	}
	assert.Equal(t, expectedEnv, c.Env)
}

func TestDeployInjectsOverrideCommand(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	cmd := model.ToUnixCmd("./foo.sh bar")
	manifest := NewSanchoDockerBuildManifest(f)
	iTarg := manifest.ImageTargetAt(0).WithOverrideCommand(cmd)
	manifest = manifest.WithImageTarget(iTarg)

	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	entities, err := k8s.ParseYAMLFromString(f.k8s.Yaml)
	if err != nil {
		t.Fatal(err)
	}

	if !assert.Equal(t, 1, len(entities)) {
		return
	}

	d := entities[0].Obj.(*v1.Deployment)
	if !assert.Equal(t, 1, len(d.Spec.Template.Spec.Containers)) {
		return
	}

	c := d.Spec.Template.Spec.Containers[0]

	// Make sure container ref injection worked as expected
	assert.Equal(t, "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95", c.Image)

	assert.Equal(t, cmd.Argv, c.Command)
	assert.Empty(t, c.Args)
}

func (f *ibdFixture) firstPodTemplateSpecHash() k8s.PodTemplateSpecHash {
	entities, err := k8s.ParseYAMLFromString(f.k8s.Yaml)
	if err != nil {
		f.T().Fatal(err)
	}

	// if you want to use this from a test that applies more than one entity, it will have to change
	require.Equal(f.T(), 1, len(entities), "expected only one entity. Yaml contained: %s", f.k8s.Yaml)

	require.IsType(f.T(), &v1.Deployment{}, entities[0].Obj)
	d := entities[0].Obj.(*v1.Deployment)
	ret := k8s.PodTemplateSpecHash(d.Spec.Template.Labels[k8s.TiltPodTemplateHashLabel])
	require.NotEqual(f.T(), ret, k8s.PodTemplateSpecHash(""))
	return ret
}

func TestDeployInjectsPodTemplateSpecHash(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	manifest := NewSanchoDockerBuildManifest(f)

	resultSet, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	hash := f.firstPodTemplateSpecHash()

	require.True(t, k8sconv.ContainsHash(resultSet.ApplyFilter(), hash))
}

func TestDeployPodTemplateSpecHashChangesWhenImageChanges(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	manifest := NewSanchoDockerBuildManifest(f)

	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	hash1 := f.firstPodTemplateSpecHash()

	// now change the image digest and build again
	f.docker.BuildOutput = docker.ExampleBuildOutput2

	_, err = f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	hash2 := f.firstPodTemplateSpecHash()

	require.NotEqual(t, hash1, hash2)
}

func TestDeployInjectOverrideCommandClearsOldCommandButNotArgs(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	cmd := model.ToUnixCmd("./foo.sh bar")
	manifest := NewSanchoDockerBuildManifestWithYaml(f, testyaml.SanchoYAMLWithCommand)
	iTarg := manifest.ImageTargetAt(0).WithOverrideCommand(cmd)
	manifest = manifest.WithImageTarget(iTarg)

	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	entities, err := k8s.ParseYAMLFromString(f.k8s.Yaml)
	if err != nil {
		t.Fatal(err)
	}

	if !assert.Equal(t, 1, len(entities)) {
		return
	}

	d := entities[0].Obj.(*v1.Deployment)
	if !assert.Equal(t, 1, len(d.Spec.Template.Spec.Containers)) {
		return
	}

	c := d.Spec.Template.Spec.Containers[0]
	assert.Equal(t, cmd.Argv, c.Command)
	assert.Equal(t, []string{"something", "something_else"}, c.Args)
}

func TestDeployInjectOverrideCommandAndArgs(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	cmd := model.ToUnixCmd("./foo.sh bar")
	manifest := NewSanchoDockerBuildManifestWithYaml(f, testyaml.SanchoYAMLWithCommand)
	iTarg := manifest.ImageTargetAt(0).WithOverrideCommand(cmd)
	iTarg.OverrideArgs = &v1alpha1.ImageMapOverrideArgs{}
	manifest = manifest.WithImageTarget(iTarg)

	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	entities, err := k8s.ParseYAMLFromString(f.k8s.Yaml)
	if err != nil {
		t.Fatal(err)
	}

	if !assert.Equal(t, 1, len(entities)) {
		return
	}

	d := entities[0].Obj.(*v1.Deployment)
	if !assert.Equal(t, 1, len(d.Spec.Template.Spec.Containers)) {
		return
	}

	c := d.Spec.Template.Spec.Containers[0]
	assert.Equal(t, cmd.Argv, c.Command)
	assert.Equal(t, []string(nil), c.Args)
}

func TestCantInjectOverrideCommandWithoutContainer(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	// CRD YAML: we WILL successfully inject the new image ref, but can't inject
	// an override command for that image because it's not in a "container" block:
	// expect an error when we try
	crdYamlWithSanchoImage := strings.ReplaceAll(testyaml.CRDYAML, testyaml.CRDImage, testyaml.SanchoImage)

	cmd := model.ToUnixCmd("./foo.sh bar")
	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(crdYamlWithSanchoImage).
		WithNamedJSONPathImageLocator("projects.example.martin-helmich.de",
			"{.spec.validation.openAPIV3Schema.properties.spec.properties.image}").
		WithImageTarget(NewSanchoDockerBuildImageTarget(f).WithOverrideCommand(cmd)).
		Build()

	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "could not inject command")
	}
}

func TestInjectOverrideCommandsMultipleImages(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	cmd1 := model.ToUnixCmd("./command1.sh foo")
	cmd2 := model.ToUnixCmd("./command2.sh bar baz")

	iTarget1 := NewSanchoDockerBuildImageTarget(f).WithOverrideCommand(cmd1)
	iTarget2 := NewSanchoSidecarDockerBuildImageTarget(f).WithOverrideCommand(cmd2)
	kTarget := k8s.MustTarget("sancho", testyaml.SanchoSidecarYAML).
		WithImageDependencies([]string{iTarget1.ImageMapName(), iTarget2.ImageMapName()})
	targets := []model.TargetSpec{iTarget1, iTarget2, kTarget}

	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	entities, err := k8s.ParseYAMLFromString(f.k8s.Yaml)
	if err != nil {
		t.Fatal(err)
	}

	if !assert.Equal(t, 1, len(entities)) {
		return
	}

	d := entities[0].Obj.(*v1.Deployment)
	if !assert.Equal(t, 2, len(d.Spec.Template.Spec.Containers)) {
		return
	}

	sanchoContainer := d.Spec.Template.Spec.Containers[0]
	sidecarContainer := d.Spec.Template.Spec.Containers[1]

	// Make sure container ref injection worked as expected
	assert.Equal(t, "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95", sanchoContainer.Image)
	assert.Equal(t, "gcr.io/some-project-162817/sancho-sidecar:tilt-11cd0b38bc3ceb95", sidecarContainer.Image)

	assert.Equal(t, cmd1.Argv, sanchoContainer.Command)
	assert.Equal(t, cmd2.Argv, sidecarContainer.Command)

}

func TestIBDDeployUIDs(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	manifest := NewSanchoDockerBuildManifest(f)
	result, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	filter := result.ApplyFilter()
	assert.Equal(t, 1, len(filter.DeployedRefs))
	assert.True(t, k8sconv.ContainsUID(filter, f.k8s.LastUpsertResult[0].UID()))
}

func TestDockerBuildTargetStage(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	iTarget := NewSanchoDockerBuildImageTarget(f)
	db := iTarget.BuildDetails.(model.DockerBuild)
	db.DockerImageSpec.Target = "stage"
	iTarget.BuildDetails = db

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoYAML).
		WithImageTargets(iTarget).
		Build()
	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "stage", f.docker.BuildOptions.Target)
}

func TestDockerBuildStatus(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	iTarget := NewSanchoDockerBuildImageTarget(f)
	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoYAML).
		WithImageTargets(iTarget).
		Build()

	iTarget = manifest.ImageTargets[0]
	nn := ktypes.NamespacedName{Name: iTarget.DockerImageName}
	err := f.ctrlClient.Create(f.ctx, &v1alpha1.DockerImage{
		ObjectMeta: metav1.ObjectMeta{Name: nn.Name},
		Spec:       iTarget.DockerBuildInfo().DockerImageSpec,
	})
	require.NoError(t, err)

	_, err = f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	require.NoError(t, err)

	var di v1alpha1.DockerImage
	err = f.ctrlClient.Get(f.ctx, nn, &di)
	require.NoError(t, err)
	require.NotNil(t, di.Status.Completed)
}

func TestCustomBuildStatus(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build"] = types.ImageInspect{ID: string(sha)}

	cb := model.CustomBuild{
		CmdImageSpec: v1alpha1.CmdImageSpec{Args: model.ToHostCmd("exit 0").Argv, OutputTag: "tilt-build"},
		Deps:         []string{f.JoinPath("app")},
	}
	iTarget := model.MustNewImageTarget(SanchoRef).WithBuildDetails(cb)
	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoYAML).
		WithImageTargets(iTarget).
		Build()

	iTarget = manifest.ImageTargets[0]
	nn := ktypes.NamespacedName{Name: iTarget.CmdImageName}
	err := f.ctrlClient.Create(f.ctx, &v1alpha1.CmdImage{
		ObjectMeta: metav1.ObjectMeta{Name: nn.Name},
		Spec:       iTarget.CustomBuildInfo().CmdImageSpec,
	})
	require.NoError(t, err)

	_, err = f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	require.NoError(t, err)

	var ci v1alpha1.CmdImage
	err = f.ctrlClient.Get(f.ctx, nn, &ci)
	require.NoError(t, err)
	require.NotNil(t, ci.Status.Completed)
}

func TestTwoManifestsWithCommonImage(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	m1, m2 := NewManifestsWithCommonAncestor(f)
	results1, err := f.BuildAndDeploy(BuildTargets(m1), store.BuildStateSet{})
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"image:gcr.io_common", "image:gcr.io_image-1", "k8s:image-1"},
		resultKeys(results1))

	stateSet := f.resultsToNextState(results1)

	results2, err := f.BuildAndDeploy(BuildTargets(m2), stateSet)
	require.NoError(t, err)
	assert.Equal(t,
		// We did not return image-common because it didn't need a rebuild.
		[]string{"image:gcr.io_image-2", "k8s:image-2"},
		resultKeys(results2))
}

func TestTwoManifestsWithCommonImagePrebuilt(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	m1, _ := NewManifestsWithCommonAncestor(f)
	iTarget1 := m1.ImageTargets[0]
	prebuilt1 := store.NewImageBuildResultSingleRef(iTarget1.ID(),
		container.MustParseNamedTagged("gcr.io/common:tilt-prebuilt"))

	stateSet := store.BuildStateSet{}
	stateSet[iTarget1.ID()] = store.NewBuildState(prebuilt1, nil, nil)

	results1, err := f.BuildAndDeploy(BuildTargets(m1), stateSet)
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"image:gcr.io_image-1", "k8s:image-1"},
		resultKeys(results1))
	assert.Contains(t, f.out.String(),
		"STEP 1/4 — Loading cached images\n     - gcr.io/common:tilt-prebuilt")
}

func TestTwoManifestsWithTwoCommonAncestors(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	m1, m2 := NewManifestsWithTwoCommonAncestors(f)
	results1, err := f.BuildAndDeploy(BuildTargets(m1), store.BuildStateSet{})
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"image:gcr.io_base", "image:gcr.io_common", "image:gcr.io_image-1", "k8s:image-1"},
		resultKeys(results1))

	stateSet := f.resultsToNextState(results1)

	results2, err := f.BuildAndDeploy(BuildTargets(m2), stateSet)
	require.NoError(t, err)
	assert.Equal(t,
		// We did not return image-common because it didn't need a rebuild.
		[]string{"image:gcr.io_image-2", "k8s:image-2"},
		resultKeys(results2))
}

func TestTwoManifestsWithSameTwoImages(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)

	m1, m2 := NewManifestsWithSameTwoImages(f)
	results1, err := f.BuildAndDeploy(BuildTargets(m1), store.BuildStateSet{})
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"image:gcr.io_common", "image:gcr.io_image-1", "k8s:dep-1"},
		resultKeys(results1))

	stateSet := f.resultsToNextState(results1)

	results2, err := f.BuildAndDeploy(BuildTargets(m2), stateSet)
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"k8s:dep-2"},
		resultKeys(results2))
}

func TestPlatformFromCluster(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductGKE)
	f.cluster.Status.Arch = "amd64"

	m := NewSanchoDockerBuildManifest(f)
	iTargetID1 := m.ImageTargets[0].ID()
	stateSet := store.BuildStateSet{
		iTargetID1: store.BuildState{FullBuildTriggered: true},
	}
	_, err := f.BuildAndDeploy(BuildTargets(m), stateSet)
	require.NoError(t, err)
	assert.Equal(t, "linux/amd64", f.docker.BuildOptions.Platform)
}

func TestDockerForMacDeploy(t *testing.T) {
	f := newIBDFixture(t, clusterid.ProductDockerDesktop)

	manifest := NewSanchoDockerBuildManifest(f)
	targets := BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}

	expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	if !strings.Contains(f.k8s.Yaml, expectedYaml) {
		t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.Yaml)
	}
}

func resultKeys(result store.BuildResultSet) []string {
	keys := []string{}
	for id := range result {
		keys = append(keys, id.String())
	}
	sort.Strings(keys)
	return keys
}

type ibdFixture struct {
	*tempdir.TempDirFixture
	out        *bufsync.ThreadSafeBuffer
	ctx        context.Context
	docker     *docker.FakeClient
	k8s        *k8s.FakeK8sClient
	ibd        *ImageBuildAndDeployer
	st         *store.TestingStore
	kl         *fakeKINDLoader
	ctrlClient ctrlclient.Client
	cluster    *v1alpha1.Cluster
}

func newIBDFixture(t *testing.T, env clusterid.Product) *ibdFixture {
	f := tempdir.NewTempDirFixture(t)
	dir := dirs.NewTiltDevDirAt(f.Path())

	dockerClient := docker.NewFakeClient()

	// Make the fake ImageExists always return true, which is the behavior we want
	// when testing the ImageBuildAndDeployer.
	dockerClient.ImageAlwaysExists = true

	out := bufsync.NewThreadSafeBuffer()
	ctx, _, ta := testutils.CtxAndAnalyticsForTest()
	ctx = logger.WithLogger(ctx, logger.NewTestLogger(out))
	kClient := k8s.NewFakeK8sClient(t)
	kl := &fakeKINDLoader{}
	clock := fakeClock{time.Date(2019, 1, 1, 1, 1, 1, 1, time.UTC)}
	kubeContext := k8s.KubeContext(fmt.Sprintf("%s-me", env))
	clusterEnv := docker.ClusterEnv(docker.Env{})
	if env == clusterid.ProductDockerDesktop {
		clusterEnv.BuildToKubeContexts = []string{string(kubeContext)}
	}
	dockerClient.FakeEnv = docker.Env(clusterEnv)

	ctrlClient := fake.NewFakeTiltClient()
	st := store.NewTestingStore()
	execer := localexec.NewFakeExecer(t)
	ibd, err := ProvideImageBuildAndDeployer(ctx, dockerClient, kClient, env, kubeContext,
		clusterEnv, dir, clock, kl, ta, ctrlClient, st, execer)
	if err != nil {
		t.Fatal(err)
	}

	cluster := &v1alpha1.Cluster{
		Status: v1alpha1.ClusterStatus{
			Connection: &v1alpha1.ClusterConnectionStatus{
				Kubernetes: &v1alpha1.KubernetesClusterConnectionStatus{
					Product: string(env),
					Context: string(kubeContext),
				},
			},
		},
	}
	ret := &ibdFixture{
		TempDirFixture: f,
		out:            out,
		ctx:            ctx,
		docker:         dockerClient,
		k8s:            kClient,
		ibd:            ibd,
		st:             st,
		kl:             kl,
		ctrlClient:     ctrlClient,
		cluster:        cluster,
	}

	return ret
}

func (f *ibdFixture) upsert(obj ctrlclient.Object) {
	err := f.ctrlClient.Create(f.ctx, obj)
	if err == nil {
		return
	}

	copy := obj.DeepCopyObject().(ctrlclient.Object)
	err = f.ctrlClient.Get(f.ctx, ktypes.NamespacedName{Name: obj.GetName()}, copy)
	assert.NoError(f.T(), err)

	obj.SetResourceVersion(copy.GetResourceVersion())

	err = f.ctrlClient.Update(f.ctx, obj)
	assert.NoError(f.T(), err)
}

func (f *ibdFixture) BuildAndDeploy(specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	if stateSet == nil {
		stateSet = store.BuildStateSet{}
	}
	iTargets, kTargets := extractImageAndK8sTargets(specs)
	for _, iTarget := range iTargets {
		if iTarget.IsLiveUpdateOnly {
			continue
		}

		im := v1alpha1.ImageMap{
			ObjectMeta: metav1.ObjectMeta{Name: iTarget.ID().Name.String()},
			Spec:       iTarget.ImageMapSpec,
		}
		state := stateSet[iTarget.ID()]
		imageBuildResult, ok := state.LastResult.(store.ImageBuildResult)
		if ok {
			im.Status = imageBuildResult.ImageMapStatus
		}
		f.upsert(&im)

		s := stateSet[iTarget.ID()]
		s.Cluster = f.cluster
		stateSet[iTarget.ID()] = s
	}
	for _, kTarget := range kTargets {
		ka := v1alpha1.KubernetesApply{
			ObjectMeta: metav1.ObjectMeta{Name: kTarget.ID().Name.String()},
			Spec:       kTarget.KubernetesApplySpec,
		}
		f.upsert(&ka)
	}
	return f.ibd.BuildAndDeploy(f.ctx, f.st, specs, stateSet)
}

func (f *ibdFixture) resultsToNextState(results store.BuildResultSet) store.BuildStateSet {
	stateSet := store.BuildStateSet{}
	for id, result := range results {
		stateSet[id] = store.NewBuildState(result, nil, nil)
	}
	return stateSet
}

func (f *ibdFixture) refs(iTarget model.ImageTarget) container.RefSet {
	f.T().Helper()
	refs, err := iTarget.Refs(f.cluster)
	require.NoErrorf(f.T(), err, "Determining refs for %s", iTarget.ID().String())
	return refs
}

func newK8sMultiEntityManifest(name string) model.Manifest {
	yaml := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s-pvc
spec: {}
status: {}

---

apiVersion: v1
kind: Deployment
metadata:
  name: %s-deployment
spec: {}
status: {}`, name, name)
	return model.Manifest{Name: model.ManifestName(name)}.WithDeployTarget(model.NewK8sTargetForTesting(yaml))
}

type fakeKINDLoader struct {
	loadCount int
}

func (kl *fakeKINDLoader) LoadToKIND(ctx context.Context, cluster *v1alpha1.Cluster, ref reference.NamedTagged) error {
	kl.loadCount++
	return nil
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }
