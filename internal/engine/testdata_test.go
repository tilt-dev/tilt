package engine

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
)

type Fixture = manifestbuilder.Fixture

const SanchoYAML = testyaml.SanchoYAML

const SanchoTwinYAML = testyaml.SanchoTwinYAML

const SanchoBaseDockerfile = `
FROM go:1.10
`

const SanchoDockerfile = `
FROM go:1.10
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`

var SanchoRef = container.MustParseSelector(testyaml.SanchoImage)
var SanchoBaseRef = container.MustParseSelector("sancho-base")
var SanchoSidecarRef = container.MustParseSelector(testyaml.SanchoSidecarImage)

func NewSanchoFastBuild(fixture Fixture) model.FastBuild {
	return model.FastBuild{
		BaseDockerfile: SanchoBaseDockerfile,
		Syncs: []model.Sync{
			model.Sync{
				LocalPath:     fixture.Path(),
				ContainerPath: "/go/src/github.com/windmilleng/sancho",
			},
		},
		Runs: model.ToRuns([]model.Cmd{
			model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}},
		}),
		Entrypoint: model.Cmd{Argv: []string{"/go/bin/sancho"}},
	}
}

func SyncStepsForApp(app string, fixture Fixture) []model.LiveUpdateSyncStep {
	return []model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{
		Source: fixture.Path(),
		Dest:   fmt.Sprintf("/go/src/github.com/windmilleng/%s", app),
	}}
}
func SanchoSyncSteps(fixture Fixture) []model.LiveUpdateSyncStep {
	return SyncStepsForApp("sancho", fixture)
}

func RunStepsForApp(app string) []model.LiveUpdateRunStep {
	return []model.LiveUpdateRunStep{model.LiveUpdateRunStep{Command: model.Cmd{Argv: []string{"go", "install", fmt.Sprintf("github.com/windmilleng/%s", app)}}}}
}

var SanchoRunSteps = RunStepsForApp("sancho")

func NewSanchoFastBuildImage(fixture Fixture) model.ImageTarget {
	fbInfo := NewSanchoFastBuild(fixture)
	return model.NewImageTarget(SanchoRef).WithBuildDetails(fbInfo)
}

func NewSanchoFastBuildManifest(f Fixture) model.Manifest {
	return manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(NewSanchoFastBuildImage(f)).
		Build()
}

func NewSanchoLiveUpdateManifest(f Fixture) model.Manifest {
	return manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
}

func NewSanchoFastBuildDCManifest(f Fixture) model.Manifest {
	return manifestbuilder.New(f, "sancho").
		WithDockerCompose().
		WithImageTarget(NewSanchoFastBuildImage(f)).
		Build()
}

func NewSanchoFastBuildManifestWithCache(fixture Fixture, paths []string) model.Manifest {
	manifest := NewSanchoFastBuildManifest(fixture)
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithCachePaths(paths))
	return manifest
}

func NewSanchoManifestWithImageInEnvVar(f Fixture) model.Manifest {
	it2 := model.NewImageTarget(container.MustParseSelector(SanchoRef.String() + "2")).WithBuildDetails(model.DockerBuild{
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

func NewSanchoCustomBuildImageTarget(fixture Fixture) model.ImageTarget {
	return NewSanchoCustomBuildImageTargetWithTag(fixture, "")
}

func NewSanchoCustomBuildImageTargetWithTag(fixture Fixture, tag string) model.ImageTarget {
	cb := model.CustomBuild{
		Command: "true",
		Deps:    []string{fixture.JoinPath("app")},
		Tag:     tag,
	}
	return model.NewImageTarget(SanchoRef).WithBuildDetails(cb)
}

func NewSanchoCustomBuildManifestWithTag(fixture Fixture, tag string) model.Manifest {
	return manifestbuilder.New(fixture, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(NewSanchoCustomBuildImageTargetWithTag(fixture, tag)).
		Build()
}

func NewSanchoCustomBuildManifestWithFastBuild(fixture Fixture) model.Manifest {
	fb := NewSanchoFastBuild(fixture)
	cb := model.CustomBuild{
		Command: "true",
		Deps:    []string{fixture.JoinPath("app")},
		Fast:    fb,
	}

	return manifestbuilder.New(fixture, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(model.NewImageTarget(SanchoRef).WithBuildDetails(cb)).
		Build()
}

func NewSanchoCustomBuildManifestWithPushDisabled(fixture Fixture) model.Manifest {
	cb := model.CustomBuild{
		Command:     "true",
		Deps:        []string{fixture.JoinPath("app")},
		DisablePush: true,
		Tag:         "tilt-build",
	}

	return manifestbuilder.New(fixture, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTarget(model.NewImageTarget(SanchoRef).WithBuildDetails(cb)).
		Build()
}

func NewSanchoDockerBuildImageTarget(f Fixture) model.ImageTarget {
	return model.NewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  f.Path(),
	})
}

func NewSanchoSyncOnlyImageTarget(f Fixture, syncs []model.LiveUpdateSyncStep) model.ImageTarget {
	lu := assembleLiveUpdate(syncs, nil, false, []string{}, f)
	return imageTargetWithLiveUpdate(NewSanchoDockerBuildImageTarget(f), lu)
}

func NewSanchoLiveUpdateImageTarget(f Fixture) model.ImageTarget {
	syncs := []model.LiveUpdateSyncStep{
		{
			Source: f.Path(),
			Dest:   "/go/src/github.com/windmilleng/sancho",
		},
	}
	runs := []model.LiveUpdateRunStep{
		{
			Command: model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}},
		},
	}

	lu := assembleLiveUpdate(syncs, runs, true, []string{}, f)
	return imageTargetWithLiveUpdate(NewSanchoDockerBuildImageTarget(f), lu)
}

func NewSanchoSidecarDockerBuildImageTarget(f Fixture) model.ImageTarget {
	iTarget := NewSanchoDockerBuildImageTarget(f)
	iTarget.ConfigurationRef = SanchoSidecarRef
	iTarget.DeploymentRef = SanchoSidecarRef.AsNamedOnly()
	return iTarget
}

func NewSanchoSidecarFastBuildImageTarget(f Fixture) model.ImageTarget {
	iTarget := NewSanchoFastBuildImage(f)
	iTarget.ConfigurationRef = SanchoSidecarRef
	iTarget.DeploymentRef = SanchoSidecarRef.AsNamedOnly()
	return iTarget
}

func NewSanchoSidecarLiveUpdateImageTarget(f Fixture) model.ImageTarget {
	iTarget := NewSanchoLiveUpdateImageTarget(f)
	iTarget.ConfigurationRef = SanchoSidecarRef
	iTarget.DeploymentRef = SanchoSidecarRef.AsNamedOnly()
	return iTarget
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

func NewSanchoDockerBuildManifestWithCache(f Fixture, paths []string) model.Manifest {
	manifest := NewSanchoDockerBuildManifest(f)
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithCachePaths(paths))
	return manifest
}

func NewSanchoDockerBuildManifestWithNestedFastBuild(fixture Fixture) model.Manifest {
	manifest := NewSanchoDockerBuildManifest(fixture)
	iTarg := manifest.ImageTargetAt(0)
	fb := NewSanchoFastBuild(fixture)
	sb := iTarg.DockerBuildInfo()
	sb.FastBuild = fb
	iTarg = iTarg.WithBuildDetails(sb)
	manifest = manifest.WithImageTarget(iTarg)
	return manifest
}

func NewSanchoDockerBuildMultiStageManifest(fixture Fixture) model.Manifest {
	baseImage := model.NewImageTarget(SanchoBaseRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.JoinPath("sancho-base"),
	})

	srcImage := model.NewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `
FROM sancho-base
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
		BuildPath: fixture.JoinPath("sancho"),
	}).WithDependencyIDs([]model.TargetID{baseImage.ID()})

	return manifestbuilder.New(fixture, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTargets(baseImage, srcImage).
		Build()
}

func NewSanchoDockerBuildMultiStageManifestWithLiveUpdate(fixture Fixture, lu model.LiveUpdate) model.Manifest {
	baseImage := model.NewImageTarget(SanchoBaseRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.JoinPath("sancho-base"),
	})

	srcImage := model.NewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `
FROM sancho-base
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
		BuildPath: fixture.JoinPath("sancho"),
	}).WithDependencyIDs([]model.TargetID{baseImage.ID()})

	return manifestbuilder.New(fixture, "sancho").
		WithK8sYAML(SanchoYAML).
		WithImageTargets(baseImage, srcImage).
		WithLiveUpdateAtIndex(lu, 1).
		Build()
}

func NewSanchoLiveUpdateMultiStageManifest(fixture Fixture) model.Manifest {
	baseImage := model.NewImageTarget(SanchoBaseRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.Path(),
	})

	srcImage := NewSanchoLiveUpdateImageTarget(fixture)
	dbInfo := srcImage.DockerBuildInfo()
	dbInfo.Dockerfile = `FROM sancho-base`

	srcImage = srcImage.
		WithBuildDetails(dbInfo).
		WithDependencyIDs([]model.TargetID{baseImage.ID()})

	kTarget := model.K8sTarget{YAML: SanchoYAML}.
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

	targetCommon := model.NewImageTarget(refCommon).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.JoinPath("common"),
	})
	target1 := model.NewImageTarget(ref1).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM ` + refCommon.String(),
		BuildPath:  fixture.JoinPath("image-1"),
	})
	target2 := model.NewImageTarget(ref2).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM ` + refCommon.String(),
		BuildPath:  fixture.JoinPath("image-2"),
	})

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
