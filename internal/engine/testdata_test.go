package engine

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Fixture = manifestbuilder.Fixture

const SanchoYAML = testyaml.SanchoYAML

const SanchoDockerfile = `
FROM go:1.10
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`

var SanchoRef = container.MustParseSelector(testyaml.SanchoImage)
var SanchoBaseRef = container.MustParseSelector("sancho-base")
var SanchoSidecarRef = container.MustParseSelector(testyaml.SanchoSidecarImage)

func SyncStepsForApp(app string, fixture Fixture) []v1alpha1.LiveUpdateSync {
	return []v1alpha1.LiveUpdateSync{
		{
			LocalPath:     ".",
			ContainerPath: fmt.Sprintf("/go/src/github.com/tilt-dev/%s", app),
		},
	}
}
func SanchoSyncSteps(fixture Fixture) []v1alpha1.LiveUpdateSync {
	return SyncStepsForApp("sancho", fixture)
}

func RunStepsForApp(app string) []v1alpha1.LiveUpdateExec {
	return []v1alpha1.LiveUpdateExec{
		v1alpha1.LiveUpdateExec{Args: []string{"go", "install", fmt.Sprintf("github.com/tilt-dev/%s", app)}},
	}
}

var SanchoRunSteps = RunStepsForApp("sancho")

func NewSanchoLiveUpdateManifest(f Fixture) model.Manifest {
	return manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
}

func NewSanchoLiveUpdateDCManifest(f Fixture) model.Manifest {
	return manifestbuilder.New(f, "sancho").
		WithDockerCompose().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
}

func NewSanchoCustomBuildManifest(fixture Fixture) model.Manifest {
	return NewSanchoCustomBuildManifestWithTag(fixture, "")
}

func NewSanchoCustomBuildImageTarget(fixture Fixture) model.ImageTarget {
	return NewSanchoCustomBuildImageTargetWithTag(fixture, "")
}

func NewSanchoCustomBuildImageTargetWithTag(fixture Fixture, tag string) model.ImageTarget {
	cb := model.CustomBuild{
		CmdImageSpec: v1alpha1.CmdImageSpec{Args: model.ToHostCmd("exit 0").Argv, OutputTag: tag},
		Deps:         []string{fixture.JoinPath("app")},
	}
	return model.MustNewImageTarget(SanchoRef).WithBuildDetails(cb)
}

func NewSanchoCustomBuildManifestWithTag(fixture Fixture, tag string) model.Manifest {
	return manifestbuilder.New(fixture, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(NewSanchoCustomBuildImageTargetWithTag(fixture, tag)).
		Build()
}

func NewSanchoDockerBuildImageTarget(f Fixture) model.ImageTarget {
	return model.MustNewImageTarget(SanchoRef).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: SanchoDockerfile,
			Context:            f.Path(),
		})
}

func NewSanchoLiveUpdate(f Fixture) v1alpha1.LiveUpdateSpec {
	syncs := []v1alpha1.LiveUpdateSync{
		{

			LocalPath:     ".",
			ContainerPath: "/go/src/github.com/tilt-dev/sancho",
		},
	}
	runs := []v1alpha1.LiveUpdateExec{
		{
			Args: []string{"go", "install", "github.com/tilt-dev/sancho"},
		},
	}

	return assembleLiveUpdate(syncs, runs, true, []string{}, f)
}

func NewSanchoLiveUpdateImageTarget(f Fixture) model.ImageTarget {
	return NewSanchoDockerBuildImageTarget(f).WithLiveUpdateSpec("sancho:sancho", NewSanchoLiveUpdate(f))
}

func NewSanchoSidecarDockerBuildImageTarget(f Fixture) model.ImageTarget {
	return NewSanchoDockerBuildImageTarget(f).MustWithRef(SanchoSidecarRef)
}

func NewSanchoSidecarLiveUpdateImageTarget(f Fixture) model.ImageTarget {
	return NewSanchoLiveUpdateImageTarget(f).MustWithRef(SanchoSidecarRef)
}

func NewSanchoDockerBuildManifest(f Fixture) model.Manifest {
	return NewSanchoDockerBuildManifestWithYaml(f, SanchoYAML)
}

func NewSanchoDockerBuildManifestWithYaml(f Fixture, yaml string) model.Manifest {
	return manifestbuilder.New(f, "sancho").
		WithK8sYAML(yaml).
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		Build()
}

func NewSanchoMultiStageImages(fixture Fixture) []model.ImageTarget {
	baseImage := model.MustNewImageTarget(SanchoBaseRef).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM golang:1.10`,
			Context:            fixture.JoinPath("sancho-base"),
		})

	srcImage := model.MustNewImageTarget(SanchoRef).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `
FROM sancho-base
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`,
			Context: fixture.JoinPath("sancho"),
		}).WithImageMapDeps([]string{baseImage.ImageMapName()})
	return []model.ImageTarget{baseImage, srcImage}
}

func NewManifestsWithCommonAncestor(fixture Fixture) (model.Manifest, model.Manifest) {
	refCommon := container.MustParseSelector("gcr.io/common")
	ref1 := container.MustParseSelector("gcr.io/image-1")
	ref2 := container.MustParseSelector("gcr.io/image-2")

	fixture.MkdirAll("common")
	fixture.MkdirAll("image-1")
	fixture.MkdirAll("image-2")

	targetCommon := model.MustNewImageTarget(refCommon).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM golang:1.10`,
			Context:            fixture.JoinPath("common"),
		})
	target1 := model.MustNewImageTarget(ref1).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM ` + refCommon.String(),
			Context:            fixture.JoinPath("image-1"),
		}).WithImageMapDeps([]string{targetCommon.ImageMapName()})
	target2 := model.MustNewImageTarget(ref2).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM ` + refCommon.String(),
			Context:            fixture.JoinPath("image-2"),
		}).WithImageMapDeps([]string{targetCommon.ImageMapName()})

	m1 := manifestbuilder.New(fixture, "image-1").
		WithK8sYAML(testyaml.Deployment("image-1", ref1.String())).
		WithImageTargets(targetCommon, target1).
		Build()
	m2 := manifestbuilder.New(fixture, "image-2").
		WithK8sYAML(testyaml.Deployment("image-2", ref2.String())).
		WithImageTargets(targetCommon, target2).
		Build()
	return m1, m2
}

func NewManifestsWithTwoCommonAncestors(fixture Fixture) (model.Manifest, model.Manifest) {
	refBase := container.MustParseSelector("gcr.io/base")
	refCommon := container.MustParseSelector("gcr.io/common")
	ref1 := container.MustParseSelector("gcr.io/image-1")
	ref2 := container.MustParseSelector("gcr.io/image-2")

	fixture.MkdirAll("base")
	fixture.MkdirAll("common")
	fixture.MkdirAll("image-1")
	fixture.MkdirAll("image-2")

	targetBase := model.MustNewImageTarget(refBase).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM golang:1.10`,
			Context:            fixture.JoinPath("base"),
		})
	targetCommon := model.MustNewImageTarget(refCommon).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM ` + refBase.String(),
			Context:            fixture.JoinPath("common"),
		}).WithImageMapDeps([]string{targetBase.ImageMapName()})
	target1 := model.MustNewImageTarget(ref1).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM ` + refCommon.String(),
			Context:            fixture.JoinPath("image-1"),
		}).WithImageMapDeps([]string{targetCommon.ImageMapName()})
	target2 := model.MustNewImageTarget(ref2).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM ` + refCommon.String(),
			Context:            fixture.JoinPath("image-2"),
		}).WithImageMapDeps([]string{targetCommon.ImageMapName()})

	m1 := manifestbuilder.New(fixture, "image-1").
		WithK8sYAML(testyaml.Deployment("image-1", ref1.String())).
		WithImageTargets(targetBase, targetCommon, target1).
		Build()
	m2 := manifestbuilder.New(fixture, "image-2").
		WithK8sYAML(testyaml.Deployment("image-2", ref2.String())).
		WithImageTargets(targetBase, targetCommon, target2).
		Build()
	return m1, m2
}

func NewManifestsWithSameTwoImages(fixture Fixture) (model.Manifest, model.Manifest) {
	refCommon := container.MustParseSelector("gcr.io/common")
	ref1 := container.MustParseSelector("gcr.io/image-1")

	fixture.MkdirAll("common")
	fixture.MkdirAll("image-1")

	targetCommon := model.MustNewImageTarget(refCommon).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM golang:1.10`,
			Context:            fixture.JoinPath("common"),
		})
	target1 := model.MustNewImageTarget(ref1).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: `FROM ` + refCommon.String(),
			Context:            fixture.JoinPath("image-1"),
		}).WithImageMapDeps([]string{targetCommon.ImageMapName()})

	m1 := manifestbuilder.New(fixture, "dep-1").
		WithK8sYAML(testyaml.Deployment("dep-1", ref1.String())).
		WithImageTargets(targetCommon, target1).
		Build()
	m2 := manifestbuilder.New(fixture, "dep-2").
		WithK8sYAML(testyaml.Deployment("dep-2", ref1.String())).
		WithImageTargets(targetCommon, target1).
		Build()
	return m1, m2
}

func assembleLiveUpdate(syncs []v1alpha1.LiveUpdateSync, runs []v1alpha1.LiveUpdateExec, shouldRestart bool, fallBackOn []string, f Fixture) v1alpha1.LiveUpdateSpec {
	restart := v1alpha1.LiveUpdateRestartStrategyNone
	if shouldRestart {
		restart = v1alpha1.LiveUpdateRestartStrategyAlways
	}
	return v1alpha1.LiveUpdateSpec{
		BasePath:  f.Path(),
		Syncs:     syncs,
		Execs:     runs,
		StopPaths: fallBackOn,
		Restart:   restart,
	}
}
