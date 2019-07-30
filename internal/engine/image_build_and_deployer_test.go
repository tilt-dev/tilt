package engine

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/dirs"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestDockerBuildWithCache(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifestWithCache(f, []string{"/root/.cache"})
	cache := "gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3"
	f.docker.Images[cache] = types.ImageInspect{}

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `FROM gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3
LABEL "tilt.cache"="0"
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildOptions.Context), expected)
}

func TestBaseDockerfileWithCache(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoFastBuildManifestWithCache(f, []string{"/root/.cache"})
	cache := "gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3"
	f.docker.Images[cache] = types.ImageInspect{}

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `FROM gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3
LABEL "tilt.cache"="0"
ADD . /
RUN ["go", "install", "github.com/windmilleng/sancho"]
ENTRYPOINT ["/go/bin/sancho"]
LABEL "tilt.buildMode"="scratch"`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildOptions.Context), expected)
}

func TestDeployTwinImages(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	sancho := NewSanchoFastBuildManifest(f)
	manifest := sancho.WithDeployTarget(sancho.K8sTarget().AppendYAML(SanchoTwinYAML))
	result, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	id := manifest.ImageTargetAt(0).ID()
	expectedImage := "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	assert.Equal(t, expectedImage, result[id].Image.String())
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
	assert.Equal(t, expectedSanchoRef, result[iTarget1.ID()].Image.String())
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSanchoRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)

	expectedSidecarRef := "gcr.io/some-project-162817/sancho-sidecar:tilt-11cd0b38bc3ceb95"
	assert.Equal(t, expectedSidecarRef, result[iTarget2.ID()].Image.String())
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
	assert.Equal(t, expectedSanchoRef, result[iTarget1.ID()].Image.String())
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSanchoRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)

	expectedSidecarRef := "gcr.io/some-project-162817/sancho-sidecar:tilt-11cd0b38bc3ceb95"
	assert.Equal(t, expectedSidecarRef, result[iTarget2.ID()].Image.String())
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedSidecarRef),
		"Expected image to appear once in YAML: %s", f.k8s.Yaml)

	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, "gcr.io/windmill-public-containers/tilt-synclet:"),
		"Expected synclet to be injected once in YAML: %s", f.k8s.Yaml)
}

func TestDeployIDInjectedAndSent(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)

	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	var deployID model.DeployID
	for _, a := range f.st.Actions {
		if deployIDAction, ok := a.(DeployIDAction); ok {
			deployID = deployIDAction.DeployID
		}
	}
	if deployID == 0 {
		t.Errorf("didn't find DeployIDAction w/ non-zero DeployID in actions: %v", f.st.Actions)
	}

	assert.True(t, strings.Count(f.k8s.Yaml, k8s.TiltDeployIDLabel) >= 1,
		"Expected TiltDeployIDLabel to appear at least once in YAML: %s", f.k8s.Yaml)
	assert.True(t, strings.Count(f.k8s.Yaml, deployID.String()) >= 1,
		"Expected DeployID %q to appear at least once in YAML: %s", deployID, f.k8s.Yaml)
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

	expectedLabelStr := fmt.Sprintf("%s: %s", k8s.ManifestNameLabel, targName)
	assert.Equalf(t, 1, strings.Count(f.k8s.Yaml, expectedLabelStr),
		"Expected \"%s\"image to appear once in YAML: %s", expectedLabelStr, f.k8s.Yaml)
}

func TestImageIsClean(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	iTargetID1 := manifest.ImageTargets[0].ID()
	result1 := store.NewImageBuildResult(iTargetID1, container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"))

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
	result1 := store.BuildResult{
		TargetID:                iTargetID1,
		Image:                   container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"),
		LiveUpdatedContainerIDs: []container.ID{container.ID("12345")},
	}

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
	assert.Equal(t, 0, f.kp.pushCount)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `
FROM docker.io/library/sancho-base:tilt-11cd0b38bc3ceb95
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
	result1 := store.NewImageBuildResult(iTargetID1, container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"))
	result2 := store.NewImageBuildResult(iTargetID2, container.MustParseNamedTagged("sancho:tilt-prebuilt2"))

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
FROM docker.io/library/sancho-base:tilt-11cd0b38bc3ceb95
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
	result1 := store.NewImageBuildResult(iTargetID1, container.MustParseNamedTagged("sancho-base:tilt-prebuilt1"))
	result2 := store.NewImageBuildResult(iTargetID2, container.MustParseNamedTagged("sancho:tilt-prebuilt2"))

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
FROM docker.io/library/sancho-base:tilt-prebuilt1
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildOptions.Context), expected)
}

func TestKINDPush(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvKIND)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount)
	assert.Equal(t, 1, f.kp.pushCount)
	assert.Equal(t, 0, f.docker.PushCount)
}

func TestCustomBuildDisablePush(t *testing.T) {
	f := newIBDFixture(t, k8s.EnvKIND)
	defer f.TearDown()
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build"] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifestWithPushDisabled(f)
	_, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	assert.NoError(t, err)

	// but we also didn't try to build or push an image
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.kp.pushCount)
	assert.Equal(t, 0, f.docker.PushCount)
}

func TestDeployUsesInjectRef(t *testing.T) {
	expectedImages := []string{"foo.com/gcr.io_some-project-162817_sancho"}
	tests := []struct {
		name           string
		manifest       func(f Fixture) model.Manifest
		expectedImages []string
	}{
		{"docker build", func(f Fixture) model.Manifest { return NewSanchoDockerBuildManifest(f) }, expectedImages},
		{"fast build", NewSanchoFastBuildManifest, expectedImages},
		{"custom build", NewSanchoCustomBuildManifest, expectedImages},
		{"live multi stage", NewSanchoLiveUpdateMultiStageManifest, append(expectedImages, "foo.com/sancho-base")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newIBDFixture(t, k8s.EnvGKE)
			defer f.TearDown()

			if test.name == "custom build" {
				sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
				f.docker.Images["foo.com/gcr.io_some-project-162817_sancho:tilt-build-1546304461"] = types.ImageInspect{ID: string(sha)}
			}

			manifest := test.manifest(f)
			var err error
			for i := range manifest.ImageTargets {
				manifest.ImageTargets[i].DeploymentRef, err = container.ReplaceRegistry("foo.com", manifest.ImageTargets[i].ConfigurationRef)
				if err != nil {
					t.Fatal(err)
				}
			}

			result, err := f.ibd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
			if err != nil {
				t.Fatal(err)
			}

			var observedImages []string
			for i := range manifest.ImageTargets {
				id := manifest.ImageTargets[i].ID()
				observedImages = append(observedImages, result[id].Image.Name())
			}

			assert.ElementsMatch(t, test.expectedImages, observedImages)
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

func TestDeployInjectOverrideCommandClearsOldCommandAndArgs(t *testing.T) {
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
	assert.Empty(t, c.Args)
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

type ibdFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	docker *docker.FakeClient
	k8s    *k8s.FakeK8sClient
	ibd    *ImageBuildAndDeployer
	st     *store.TestingStore
	kp     *fakeKINDPusher
}

func newIBDFixture(t *testing.T, env k8s.Env) *ibdFixture {
	f := tempdir.NewTempDirFixture(t)
	dir := dirs.NewWindmillDirAt(f.Path())
	docker := docker.NewFakeClient()
	ctx, _, ta := testutils.CtxAndAnalyticsForTest()
	kClient := k8s.NewFakeK8sClient()
	kp := &fakeKINDPusher{}
	clock := fakeClock{time.Date(2019, 1, 1, 1, 1, 1, 1, time.UTC)}
	ibd, err := provideImageBuildAndDeployer(ctx, docker, kClient, env, dir, clock, kp, ta)
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
		kp:             kp,
	}
}

func (f *ibdFixture) TearDown() {
	f.k8s.TearDown()
	f.TempDirFixture.TearDown()
}

type fakeKINDPusher struct {
	pushCount int
}

func (kp *fakeKINDPusher) PushToKIND(ctx context.Context, ref reference.NamedTagged, w io.Writer) error {
	kp.pushCount++
	return nil
}
