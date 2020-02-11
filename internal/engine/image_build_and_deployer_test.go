package engine

import (
	"archive/tar"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/windmilleng/wmclient/pkg/dirs"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestDeployTwinImages(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	sancho := NewSanchoDockerBuildManifest(f)
	manifest := sancho.WithDeployTarget(sancho.K8sTarget().AppendYAML(SanchoTwinYAML))
	result, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	id := manifest.ImageTargetAt(0).ID()
	expectedImage := "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	image := store.ClusterImageRefFromBuildResult(result[id])
	assert.Equal(t, expectedImage, image.String())
	assert.Equalf(t, 2, strings.Count(f.k8s.Yaml, expectedImage),
		"Expected image to update twice in YAML: %s", f.k8s.Yaml)
}

func TestDeployPodWithMultipleImages(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	iTarget1 := NewSanchoDockerBuildImageTarget(f)
	iTarget2 := NewSanchoSidecarDockerBuildImageTarget(f)
	kTarget := model.K8sTarget{Name: "sancho", YAML: testyaml.SanchoSidecarYAML}.
		WithDependencyIDs([]model.TargetID{iTarget1.ID(), iTarget2.ID()})
	targets := []model.TargetSpec{iTarget1, iTarget2, kTarget}

	result, err := f.ibd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)

	expectedSanchoRef := "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	image := store.ClusterImageRefFromBuildResult(result[iTarget1.ID()])
	assert.Equal(t, expectedSanchoRef, image.String())
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSanchoRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)

	expectedSidecarRef := "gcr.io/some-project-162817/sancho-sidecar:tilt-11cd0b38bc3ceb95"
	image = store.ClusterImageRefFromBuildResult(result[iTarget2.ID()])
	assert.Equal(t, expectedSidecarRef, image.String())
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSidecarRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)
}

func TestDeployPodWithMultipleLiveUpdateImages(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()
	f.ibd.injectSynclet = true

	iTarget1 := NewSanchoLiveUpdateImageTarget(f)
	iTarget2 := NewSanchoSidecarLiveUpdateImageTarget(f)

	kTarget := model.K8sTarget{Name: "sancho", YAML: testyaml.SanchoSidecarYAML}.
		WithDependencyIDs([]model.TargetID{iTarget1.ID(), iTarget2.ID()})
	targets := []model.TargetSpec{iTarget1, iTarget2, kTarget}

	result, err := f.ibd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)

	expectedSanchoRef := "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	image := store.ClusterImageRefFromBuildResult(result[iTarget1.ID()])
	assert.Equal(t, expectedSanchoRef, image.String())
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSanchoRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)

	expectedSidecarRef := "gcr.io/some-project-162817/sancho-sidecar:tilt-11cd0b38bc3ceb95"
	image = store.ClusterImageRefFromBuildResult(result[iTarget2.ID()])
	assert.Equal(t, expectedSidecarRef, image.String())
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSidecarRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)

	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, "gcr.io/windmill-public-containers/tilt-synclet:"),
		"Expected synclet to be injected once in YAML: %s", f.k8s.Yaml)
}

func TestNoImageTargets(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	targName := "some-k8s-manifest"
	specs := []model.TargetSpec{
		model.K8sTarget{
			Name: model.TargetName(targName),
			YAML: testyaml.LonelyPodYAML,
		},
	}

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, specs, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount, "expect no docker builds")
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, "image: gcr.io/windmill-public-containers/lonely-pod"),
		"Expected lonely-pod image to appear once in YAML: %s", f.k8s.Yaml)

	expectedLabelStr := fmt.Sprintf("%s: %s", k8s.ManagedByLabel, k8s.ManagedByValue)
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedLabelStr),
		"Expected \"%s\" label to appear once in YAML: %s", expectedLabelStr, f.k8s.Yaml)
}

func TestImageIsClean(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	iTargetID1 := manifest.ImageTargets[0].ID()
	result1 := store.NewImageBuildResultSingleRef(iTargetID1, container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"))

	f.docker.ImageListCount = 1

	stateSet := store.BuildStateSet{
		iTargetID1: store.NewBuildState(result1, []string{}),
	}
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	// Expect no build or push, b/c image is clean (i.e. last build was an image build and
	// no file changes since).
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
}

func TestImageIsDirtyAfterContainerBuild(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	iTargetID1 := manifest.ImageTargets[0].ID()
	result1 := store.NewLiveUpdateBuildResult(
		iTargetID1,
		[]container.ID{container.ID("12345")})

	stateSet := store.BuildStateSet{
		iTargetID1: store.NewBuildState(result1, []string{}),
	}
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	// Expect build + push; last result has a container ID, which implies that it was an in-place
	// update, so the current state of this manifest is NOT reflected in an existing image.
	assert.Equal(t, 1, f.docker.BuildCount)
	assert.Equal(t, 1, f.docker.PushCount)
}

func TestMultiStageDockerBuild(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildMultiStageManifest(f)
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 1, f.docker.PushCount)
	assert.Equal(t, 0, f.kl.loadCount)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `
FROM sancho-base:tilt-11cd0b38bc3ceb95
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildOptions.Context), expected)
}

func TestMultiStageDockerBuildPreservesSyntaxDirective(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	baseImage := model.MustNewImageTarget(SanchoBaseRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  f.JoinPath("sancho-base"),
	})

	srcImage := model.MustNewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `# syntax = docker/dockerfile:experimental

FROM sancho-base
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
		BuildPath: f.JoinPath("sancho"),
	}).WithDependencyIDs([]model.TargetID{baseImage.ID()})

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTargets(baseImage, srcImage).
		Build()

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(m), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 1, f.docker.PushCount)
	assert.Equal(t, 0, f.kl.loadCount)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `# syntax = docker/dockerfile:experimental

FROM sancho-base:tilt-11cd0b38bc3ceb95
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildOptions.Context), expected)
}

func TestMultiStageDockerBuildWithFirstImageDirty(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildMultiStageManifest(f)
	iTargetID1 := manifest.ImageTargets[0].ID()
	iTargetID2 := manifest.ImageTargets[1].ID()
	result1 := store.NewImageBuildResultSingleRef(iTargetID1, container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"))
	result2 := store.NewImageBuildResultSingleRef(iTargetID2, container.MustParseNamedTagged("sancho:tilt-prebuilt2"))

	newFile := f.WriteFile("sancho-base/message.txt", "message")

	stateSet := store.BuildStateSet{
		iTargetID1: store.NewBuildState(result1, []string{newFile}),
		iTargetID2: store.NewBuildState(result2, nil),
	}
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 1, f.docker.PushCount)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `
FROM sancho-base:tilt-11cd0b38bc3ceb95
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildOptions.Context), expected)
}

func TestMultiStageDockerBuildWithSecondImageDirty(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildMultiStageManifest(f)
	iTargetID1 := manifest.ImageTargets[0].ID()
	iTargetID2 := manifest.ImageTargets[1].ID()
	result1 := store.NewImageBuildResultSingleRef(iTargetID1, container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"))
	result2 := store.NewImageBuildResultSingleRef(iTargetID2, container.MustParseNamedTagged("sancho:tilt-prebuilt2"))

	newFile := f.WriteFile("sancho/message.txt", "message")

	f.docker.ImageListCount = 1

	stateSet := store.BuildStateSet{
		iTargetID1: store.NewBuildState(result1, nil),
		iTargetID2: store.NewBuildState(result2, []string{newFile}),
	}
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `
FROM sancho-base:tilt-prebuilt1
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildOptions.Context), expected)
}

func TestKINDLoad(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvKIND6)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount)
	assert.Equal(t, 1, f.kl.loadCount)
	assert.Equal(t, 0, f.docker.PushCount)
}

func TestDockerPushIfKINDAndClusterRef(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvKIND6)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	iTarg := manifest.ImageTargetAt(0)
	iTarg.Refs = iTarg.Refs.MustWithRegistry(container.MustNewRegistryWithHostFromCluster("localhost:1234", "registry:1234"))
	manifest = manifest.WithImageTarget(iTarg)

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount, "Docker build count")
	assert.Equal(t, 0, f.kl.loadCount, "KIND load count")
	assert.Equal(t, 1, f.docker.PushCount, "Docker push count")
	assert.Equal(t, iTarg.Refs.LocalRef().String(), container.MustParseNamed(f.docker.PushImage).Name(), "image pushed to Docker as LocalRef")

	yaml := f.k8s.Yaml
	assert.Contains(t, yaml, iTarg.Refs.ClusterRef().String(), "ClusterRef was injected into applied YAML")
	assert.NotContains(t, yaml, iTarg.Refs.LocalRef().String(), "LocalRef was NOT injected into applied YAML")
}

func TestCustomBuildDisablePush(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvKIND6)
	defer f.TearDown()
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build"] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifestWithPushDisabled(f)
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	assert.NoError(t, err)

	// We didn't try to build or push an image, but we did try to tag it
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 1, f.docker.TagCount)
	assert.Equal(t, 0, f.kl.loadCount)
	assert.Equal(t, 0, f.docker.PushCount)
}

func TestCustomBuildSkipsLocalDocker(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvKIND6)
	defer f.TearDown()
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build"] = types.ImageInspect{ID: string(sha)}

	cb := model.CustomBuild{
		Command:          "true",
		Deps:             []string{f.JoinPath("app")},
		SkipsLocalDocker: true,
		Tag:              "tilt-build",
	}

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(model.MustNewImageTarget(SanchoRef).WithBuildDetails(cb)).
		Build()

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
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
		{"docker build", func(f Fixture) model.Manifest { return NewSanchoDockerBuildManifest(f) }, false, expectedImages, expectedImages},
		{"docker build + distinct clusterRef", func(f Fixture) model.Manifest { return NewSanchoDockerBuildManifest(f) }, true, expectedImages, expectedImagesClusterRef},
		{"custom build", NewSanchoCustomBuildManifest, false, expectedImages, expectedImages},
		{"custom build + distinct clusterRef", NewSanchoCustomBuildManifest, true, expectedImages, expectedImagesClusterRef},
		{"live multi stage", NewSanchoLiveUpdateMultiStageManifest, false, append(expectedImages, "foo.com/sancho-base"), expectedImages},
		{"live multi stage + distinct clusterRef", NewSanchoLiveUpdateMultiStageManifest, true, append(expectedImages, "foo.com/sancho-base"), expectedImagesClusterRef},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newIBDFixture(t, k8s.EnvGKE)
			defer f.TearDown()

			if strings.Contains(test.name, "custom build") {
				sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
				f.docker.Images["foo.com/gcr.io_some-project-162817_sancho:tilt-build-1546304461"] = types.ImageInspect{ID: string(sha)}
			}

			manifest := test.manifest(f)
			for i := range manifest.ImageTargets {
				reg := container.MustNewRegistry("foo.com")
				if test.withClusterRef {
					reg = container.MustNewRegistryWithHostFromCluster("foo.com", "registry:1234")
				}
				manifest.ImageTargets[i].Refs = manifest.ImageTargets[i].Refs.MustWithRegistry(reg)
			}

			result, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
			if err != nil {
				t.Fatal(err)
			}

			var observedImages []string
			for i := range manifest.ImageTargets {
				id := manifest.ImageTargets[i].ID()
				image := store.LocalImageRefFromBuildResult(result[id])
				observedImages = append(observedImages, image.Name())
			}

			assert.ElementsMatch(t, test.expectBuilt, observedImages)

			for _, expected := range test.expectDeployed {
				assert.Contains(t, f.k8s.Yaml, expected)
			}
		})
	}
}

func TestDeployInjectImageEnvVar(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoManifestWithImageInEnvVar(f)
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
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
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	cmd := model.ToShellCmd("./foo.sh bar")
	manifest := NewSanchoDockerBuildManifest(f)
	iTarg := manifest.ImageTargetAt(0).WithOverrideCommand(cmd)
	manifest = manifest.WithImageTarget(iTarg)

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
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
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)

	resultSet, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	hash := f.firstPodTemplateSpecHash()

	require.True(t, resultSet.DeployedPodTemplateSpecHashes().Contains(hash))
}

func TestDeployPodTemplateSpecHashChangesWhenImageChanges(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	hash1 := f.firstPodTemplateSpecHash()

	// now change the image digest and build again
	f.docker.BuildOutput = docker.ExampleBuildOutput2

	_, err = f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	hash2 := f.firstPodTemplateSpecHash()

	require.NotEqual(t, hash1, hash2)
}

func TestDeployInjectOverrideCommandClearsOldCommandButNotArgs(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	cmd := model.ToShellCmd("./foo.sh bar")
	manifest := NewSanchoDockerBuildManifestWithYaml(f, testyaml.SanchoYAMLWithCommand)
	iTarg := manifest.ImageTargetAt(0).WithOverrideCommand(cmd)
	manifest = manifest.WithImageTarget(iTarg)

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
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
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	cmd := model.ToShellCmd("./foo.sh bar")
	manifest := NewSanchoDockerBuildManifestWithYaml(f, testyaml.SanchoYAMLWithCommand)
	iTarg := manifest.ImageTargetAt(0).WithOverrideCommand(cmd)
	iTarg.OverrideArgs = model.OverrideArgs{ShouldOverride: true}
	manifest = manifest.WithImageTarget(iTarg)

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
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
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	// CRD YAML: we WILL successfully inject the new image ref, but can't inject
	// an override command for that image because it's not in a "container" block:
	// expect an error when we try
	crdYamlWithSanchoImage := strings.ReplaceAll(testyaml.CRDYAML, testyaml.CRDImage, testyaml.SanchoImage)

	cmd := model.ToShellCmd("./foo.sh bar")
	manifest := NewSanchoDockerBuildManifestWithYaml(f, crdYamlWithSanchoImage)
	iTarg := manifest.ImageTargetAt(0).WithOverrideCommand(cmd)
	manifest = manifest.WithImageTarget(iTarg)

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "could not inject command")
	}
}

func TestInjectOverrideCommandsMultipleImages(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	cmd1 := model.ToShellCmd("./command1.sh foo")
	cmd2 := model.ToShellCmd("./command2.sh bar baz")

	iTarget1 := NewSanchoDockerBuildImageTarget(f).WithOverrideCommand(cmd1)
	iTarget2 := NewSanchoSidecarDockerBuildImageTarget(f).WithOverrideCommand(cmd2)
	kTarget := model.K8sTarget{Name: "sancho", YAML: testyaml.SanchoSidecarYAML}.
		WithDependencyIDs([]model.TargetID{iTarget1.ID(), iTarget2.ID()})
	targets := []model.TargetSpec{iTarget1, iTarget2, kTarget}

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
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
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	result, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(result.DeployedUIDSet()))
	assert.True(t, result.DeployedUIDSet().Contains(f.k8s.LastUpsertResult[0].UID()))
}

func TestDockerBuildTargetStage(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	iTarget := NewSanchoDockerBuildImageTarget(f)
	db := iTarget.BuildDetails.(model.DockerBuild)
	db.TargetStage = "stage"
	iTarget.BuildDetails = db

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoYAML).
		WithImageTargets(iTarget).
		Build()
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "stage", f.docker.BuildOptions.Target)
}

type ibdFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	docker *docker.FakeClient
	k8s    *k8s.FakeK8sClient
	ibd    *ImageBuildAndDeployer
	st     *store.TestingStore
	kl     *fakeKINDLoader
}

func newIBDFixture(t *testing.T, env k8s.Env) *ibdFixture {
	f := tempdir.NewTempDirFixture(t)
	dir := dirs.NewWindmillDirAt(f.Path())
	docker := docker.NewFakeClient()
	ctx, _, ta := testutils.CtxAndAnalyticsForTest()
	kClient := k8s.NewFakeK8sClient()
	kl := &fakeKINDLoader{}
	clock := fakeClock{time.Date(2019, 1, 1, 1, 1, 1, 1, time.UTC)}
	ibd, err := provideImageBuildAndDeployer(ctx, docker, kClient, env, dir, clock, kl, ta)
	if err != nil {
		t.Fatal(err)
	}
	return &ibdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		docker:         docker,
		k8s:            kClient,
		ibd:            ibd,
		st:             store.NewTestingStore(),
		kl:             kl,
	}
}

func (f *ibdFixture) TearDown() {
	f.k8s.TearDown()
	f.TempDirFixture.TearDown()
}

func (f *ibdFixture) replaceRegistry(defaultReg string, sel container.RefSelector) reference.Named {
	reg := container.MustNewRegistry(defaultReg)
	named, err := reg.ReplaceRegistryForLocalRef(sel)
	if err != nil {
		f.T().Fatal(err)
	}
	return named
}

type fakeKINDLoader struct {
	loadCount int
}

func (kl *fakeKINDLoader) LoadToKIND(ctx context.Context, ref reference.NamedTagged) error {
	kl.loadCount++
	return nil
}
