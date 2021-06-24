package buildcontrol

import (
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Fixture = manifestbuilder.Fixture

var testImageRef = container.MustParseNamedTagged("gcr.io/some-project-162817/sancho:deadbeef")
var imageTargetID = model.TargetID{
	Type: model.TargetTypeImage,
	Name: model.TargetName(apis.SanitizeName("gcr.io/some-project-162817/sancho")),
}

var alreadyBuilt = store.NewImageBuildResultSingleRef(imageTargetID, testImageRef)

const SanchoYAML = testyaml.SanchoYAML

const SanchoTwinYAML = testyaml.SanchoTwinYAML

const SanchoDockerfile = `
FROM go:1.10
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`

var SanchoRef = container.MustParseSelector(testyaml.SanchoImage)
var SanchoBaseRef = container.MustParseSelector("sancho-base")
var SanchoSidecarRef = container.MustParseSelector(testyaml.SanchoSidecarImage)

func NewSanchoLiveUpdateManifest(f Fixture) model.Manifest {
	return manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
}

func NewSanchoManifestWithImageInEnvVar(f Fixture) model.Manifest {
	it2 := model.MustNewImageTarget(container.MustParseSelector(SanchoRef.String() + "2")).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  f.Path(),
	})
	it2.MatchInEnvVars = true
	return manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoImageInEnvYAML).
		WithImageTargets(NewSanchoDockerBuildImageTarget(f), it2).
		Build()
}

func NewSanchoCustomBuildManifest(fixture Fixture) model.Manifest {
	return NewSanchoCustomBuildManifestWithTag(fixture, "")
}

func NewSanchoCustomBuildImageTargetWithTag(fixture Fixture, tag string) model.ImageTarget {
	cb := model.CustomBuild{
		Command: model.ToHostCmd("exit 0"),
		Deps:    []string{fixture.JoinPath("app")},
		Tag:     tag,
	}
	return model.MustNewImageTarget(SanchoRef).WithBuildDetails(cb)
}

func NewSanchoCustomBuildManifestWithTag(fixture Fixture, tag string) model.Manifest {
	return manifestbuilder.New(fixture, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(NewSanchoCustomBuildImageTargetWithTag(fixture, tag)).
		Build()
}

func NewSanchoCustomBuildManifestWithPushDisabled(fixture Fixture) model.Manifest {
	cb := model.CustomBuild{
		Command:     model.ToHostCmd("exit 0"),
		Deps:        []string{fixture.JoinPath("app")},
		DisablePush: true,
		Tag:         "tilt-build",
	}

	return manifestbuilder.New(fixture, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(model.MustNewImageTarget(SanchoRef).WithBuildDetails(cb)).
		Build()
}

func NewSanchoDockerBuildImageTarget(f Fixture) model.ImageTarget {
	return model.MustNewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  f.Path(),
	})
}

func NewSanchoLiveUpdate(f Fixture) model.LiveUpdate {
	syncs := []model.LiveUpdateSyncStep{
		{
			Source: f.Path(),
			Dest:   "/go/src/github.com/tilt-dev/sancho",
		},
	}
	runs := []model.LiveUpdateRunStep{
		{
			Command: model.Cmd{Argv: []string{"go", "install", "github.com/tilt-dev/sancho"}},
		},
	}

	return assembleLiveUpdate(syncs, runs, true, []string{}, f)
}

func NewSanchoLiveUpdateImageTarget(f Fixture) model.ImageTarget {
	return imageTargetWithLiveUpdate(NewSanchoDockerBuildImageTarget(f), NewSanchoLiveUpdate(f))
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

func NewSanchoDockerBuildMultiStageManifest(fixture Fixture) model.Manifest {
	baseImage := model.MustNewImageTarget(SanchoBaseRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.JoinPath("sancho-base"),
	})

	srcImage := model.MustNewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `
FROM sancho-base
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`,
		BuildPath: fixture.JoinPath("sancho"),
	}).WithDependencyIDs([]model.TargetID{baseImage.ID()})

	return manifestbuilder.New(fixture, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTargets(baseImage, srcImage).
		Build()
}

func NewSanchoLiveUpdateMultiStageManifest(fixture Fixture) model.Manifest {
	baseImage := model.MustNewImageTarget(SanchoBaseRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.Path(),
	})

	srcImage := NewSanchoLiveUpdateImageTarget(fixture)
	dbInfo := srcImage.DockerBuildInfo()
	dbInfo.Dockerfile = `FROM sancho-base`

	srcImage = srcImage.
		WithBuildDetails(dbInfo).
		WithDependencyIDs([]model.TargetID{baseImage.ID()})

	kTarget := k8s.MustTarget("sancho", SanchoYAML).
		WithDependencyIDs([]model.TargetID{srcImage.ID()})

	return model.Manifest{Name: "sancho"}.
		WithImageTargets([]model.ImageTarget{baseImage, srcImage}).
		WithDeployTarget(kTarget)
}

func NewManifestsWithCommonAncestor(fixture Fixture) (model.Manifest, model.Manifest) {
	refCommon := container.MustParseSelector("gcr.io/common")
	ref1 := container.MustParseSelector("gcr.io/image-1")
	ref2 := container.MustParseSelector("gcr.io/image-2")

	fixture.MkdirAll("common")
	fixture.MkdirAll("image-1")
	fixture.MkdirAll("image-2")

	targetCommon := model.MustNewImageTarget(refCommon).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.JoinPath("common"),
	})
	target1 := model.MustNewImageTarget(ref1).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM ` + refCommon.String(),
		BuildPath:  fixture.JoinPath("image-1"),
	}).WithDependencyIDs([]model.TargetID{targetCommon.ID()})
	target2 := model.MustNewImageTarget(ref2).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM ` + refCommon.String(),
		BuildPath:  fixture.JoinPath("image-2"),
	}).WithDependencyIDs([]model.TargetID{targetCommon.ID()})

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

	targetBase := model.MustNewImageTarget(refBase).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.JoinPath("base"),
	})
	targetCommon := model.MustNewImageTarget(refCommon).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM ` + refBase.String(),
		BuildPath:  fixture.JoinPath("common"),
	}).WithDependencyIDs([]model.TargetID{targetBase.ID()})
	target1 := model.MustNewImageTarget(ref1).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM ` + refCommon.String(),
		BuildPath:  fixture.JoinPath("image-1"),
	}).WithDependencyIDs([]model.TargetID{targetCommon.ID()})
	target2 := model.MustNewImageTarget(ref2).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM ` + refCommon.String(),
		BuildPath:  fixture.JoinPath("image-2"),
	}).WithDependencyIDs([]model.TargetID{targetCommon.ID()})

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

	targetCommon := model.MustNewImageTarget(refCommon).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.JoinPath("common"),
	})
	target1 := model.MustNewImageTarget(ref1).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM ` + refCommon.String(),
		BuildPath:  fixture.JoinPath("image-1"),
	}).WithDependencyIDs([]model.TargetID{targetCommon.ID()})

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

func assembleLiveUpdate(syncs []model.LiveUpdateSyncStep, runs []model.LiveUpdateRunStep, shouldRestart bool, fallBackOn []string, f Fixture) model.LiveUpdate {
	var steps []model.LiveUpdateStep
	if len(fallBackOn) > 0 {
		steps = append(steps, model.LiveUpdateFallBackOnStep{Files: fallBackOn})
	}
	for _, sync := range syncs {
		steps = append(steps, sync)
	}
	for _, run := range runs {
		steps = append(steps, run)
	}
	if shouldRestart {
		steps = append(steps, model.LiveUpdateRestartContainerStep{})
	}
	lu, err := model.NewLiveUpdate(steps, f.Path())
	if err != nil {
		f.T().Fatal(err)
	}
	return lu
}

func imageTargetWithLiveUpdate(i model.ImageTarget, lu model.LiveUpdate) model.ImageTarget {
	db := i.DockerBuildInfo()
	db.LiveUpdate = lu
	return i.WithBuildDetails(db)
}
