package engine

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
)

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

type pather interface {
	Path() string
	JoinPath(ps ...string) string
	MkdirAll(p string)
}

var SanchoRef = container.MustParseSelector(testyaml.SanchoImage)
var SanchoBaseRef = container.MustParseSelector("sancho-base")
var SanchoSidecarRef = container.MustParseSelector(testyaml.SanchoSidecarImage)

func NewSanchoFastBuild(fixture pather) model.FastBuild {
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

func SanchoSyncSteps(fixture pather) []model.LiveUpdateSyncStep {
	return []model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{
		Source: fixture.Path(),
		Dest:   "/go/src/github.com/windmilleng/sancho",
	}}
}

var SanchoRunSteps = []model.LiveUpdateRunStep{model.LiveUpdateRunStep{Command: model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}}}}

func NewSanchoFastBuildImage(fixture pather) model.ImageTarget {
	fbInfo := NewSanchoFastBuild(fixture)
	return model.NewImageTarget(SanchoRef).WithBuildDetails(fbInfo)
}

func NewSanchoFastBuildManifest(fixture pather) model.Manifest {
	return assembleK8sManifest(
		model.Manifest{Name: "sancho"},
		model.K8sTarget{YAML: SanchoYAML},
		NewSanchoFastBuildImage(fixture))
}

func NewSanchoLiveUpdateManifest(fixture pather) model.Manifest {
	return assembleK8sManifest(
		model.Manifest{Name: "sancho"},
		model.K8sTarget{YAML: SanchoYAML},
		NewSanchoLiveUpdateImageTarget(fixture))
}

func NewSanchoFastBuildDCManifest(fixture pather) model.Manifest {
	return assembleDCManifest(
		model.Manifest{Name: "sancho"},
		NewSanchoFastBuildImage(fixture))
}

func NewSanchoFastBuildManifestWithCache(fixture pather, paths []string) model.Manifest {
	manifest := NewSanchoFastBuildManifest(fixture)
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithCachePaths(paths))
	return manifest
}

func NewSanchoManifestWithImageInEnvVar(f pather) model.Manifest {
	it2 := model.NewImageTarget(container.MustParseSelector(SanchoRef.String() + "2")).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  f.Path(),
	})
	it2.MatchInEnvVars = true
	return assembleK8sManifest(
		model.Manifest{Name: "sancho"},
		model.K8sTarget{YAML: testyaml.SanchoImageInEnvYAML},
		NewSanchoDockerBuildImageTarget(f),
		it2,
	)
}

func NewSanchoCustomBuildManifest(fixture pather) model.Manifest {
	return NewSanchoCustomBuildManifestWithTag(fixture, "")
}

func NewSanchoCustomBuildManifestWithTag(fixture pather, tag string) model.Manifest {
	cb := model.CustomBuild{
		Command: "true",
		Deps:    []string{fixture.JoinPath("app")},
		Tag:     tag,
	}

	m := model.Manifest{Name: "sancho"}

	return assembleK8sManifest(
		m,
		model.K8sTarget{YAML: SanchoYAML},
		model.NewImageTarget(SanchoRef).WithBuildDetails(cb))
}

func NewSanchoCustomBuildManifestWithFastBuild(fixture pather) model.Manifest {
	fb := NewSanchoFastBuild(fixture)
	cb := model.CustomBuild{
		Command: "true",
		Deps:    []string{fixture.JoinPath("app")},
		Fast:    fb,
	}

	m := model.Manifest{Name: "sancho"}

	return assembleK8sManifest(
		m,
		model.K8sTarget{YAML: SanchoYAML},
		model.NewImageTarget(SanchoRef).WithBuildDetails(cb))
}

func NewSanchoCustomBuildManifestWithPushDisabled(fixture pather) model.Manifest {
	cb := model.CustomBuild{
		Command:     "true",
		Deps:        []string{fixture.JoinPath("app")},
		DisablePush: true,
		Tag:         "tilt-build",
	}

	m := model.Manifest{Name: "sancho"}

	return assembleK8sManifest(
		m,
		model.K8sTarget{YAML: SanchoYAML},
		model.NewImageTarget(SanchoRef).WithBuildDetails(cb))
}

func NewSanchoDockerBuildImageTarget(f pather) model.ImageTarget {
	return model.NewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  f.Path(),
	})
}

func NewSanchoLiveUpdateImageTarget(f pather) model.ImageTarget {
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

	lu, err := assembleLiveUpdate(syncs, runs, true, []string{}, f)
	if err != nil {
		panic(fmt.Sprintf("making sancho LiveUpdate image target: %v", err))
	}
	return imageTargetWithLiveUpdate(model.NewImageTarget(SanchoRef), lu)
}

func NewSanchoSidecarDockerBuildImageTarget(f pather) model.ImageTarget {
	iTarget := NewSanchoDockerBuildImageTarget(f)
	iTarget.ConfigurationRef = SanchoSidecarRef
	iTarget.DeploymentRef = SanchoSidecarRef.AsNamedOnly()
	return iTarget
}

func NewSanchoSidecarFastBuildImageTarget(f pather) model.ImageTarget {
	iTarget := NewSanchoFastBuildImage(f)
	iTarget.ConfigurationRef = SanchoSidecarRef
	iTarget.DeploymentRef = SanchoSidecarRef.AsNamedOnly()
	return iTarget
}

func NewSanchoSidecarLiveUpdateImageTarget(f pather) model.ImageTarget {
	iTarget := NewSanchoLiveUpdateImageTarget(f)
	iTarget.ConfigurationRef = SanchoSidecarRef
	iTarget.DeploymentRef = SanchoSidecarRef.AsNamedOnly()
	return iTarget
}

func NewSanchoDockerBuildManifest(f pather) model.Manifest {
	return NewSanchoDockerBuildManifestWithYaml(f, SanchoYAML)
}

func NewSanchoDockerBuildManifestWithYaml(f pather, yaml string) model.Manifest {
	return assembleK8sManifest(
		model.Manifest{Name: "sancho"},
		model.K8sTarget{YAML: yaml},
		NewSanchoDockerBuildImageTarget(f))
}

func NewSanchoDockerBuildManifestWithCache(f pather, paths []string) model.Manifest {
	manifest := NewSanchoDockerBuildManifest(f)
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithCachePaths(paths))
	return manifest
}

func NewSanchoDockerBuildManifestWithNestedFastBuild(fixture pather) model.Manifest {
	manifest := NewSanchoDockerBuildManifest(fixture)
	iTarg := manifest.ImageTargetAt(0)
	fb := NewSanchoFastBuild(fixture)
	sb := iTarg.DockerBuildInfo()
	sb.FastBuild = fb
	iTarg = iTarg.WithBuildDetails(sb)
	manifest = manifest.WithImageTarget(iTarg)
	return manifest
}

func NewSanchoDockerBuildMultiStageManifest(fixture pather) model.Manifest {
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

	kTarget := model.K8sTarget{YAML: SanchoYAML}.
		WithDependencyIDs([]model.TargetID{srcImage.ID()})

	return model.Manifest{Name: "sancho"}.
		WithImageTargets([]model.ImageTarget{baseImage, srcImage}).
		WithDeployTarget(kTarget)
}

func NewSanchoLiveUpdateMultiStageManifest(fixture pather) model.Manifest {
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

func NewManifestsWithCommonAncestor(fixture pather) (model.Manifest, model.Manifest) {
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

	m1 := assembleK8sManifest(
		model.Manifest{Name: "image-1"},
		model.K8sTarget{YAML: testyaml.Deployment("image-1", ref1.String())},
		targetCommon, target1)
	m2 := assembleK8sManifest(
		model.Manifest{Name: "image-2"},
		model.K8sTarget{YAML: testyaml.Deployment("image-2", ref2.String())},
		targetCommon, target2)
	return m1, m2
}

// Assemble these targets into a manifest, that deploys to k8s,
// wiring up all the dependency ids so that the K8sTarget depends on all
// the image targets
func assembleK8sManifest(m model.Manifest, k model.K8sTarget, iTargets ...model.ImageTarget) model.Manifest {
	ids := make([]model.TargetID, 0, len(iTargets))
	for _, iTarget := range iTargets {
		ids = append(ids, iTarget.ID())
	}
	k = k.WithDependencyIDs(ids)
	return m.
		WithImageTargets(iTargets).
		WithDeployTarget(k)
}

// Assemble these targets into a manifest, that deploys to docker compose,
// wiring up all the dependency ids so that the DockerComposeTarget depends on all
// the image targets
func assembleDCManifest(m model.Manifest, iTargets ...model.ImageTarget) model.Manifest {
	ids := make([]model.TargetID, 0, len(iTargets))
	for _, iTarget := range iTargets {
		ids = append(ids, iTarget.ID())
	}
	dc := dcTarg.WithDependencyIDs(ids)
	return m.
		WithImageTargets(iTargets).
		WithDeployTarget(dc)
}

func assembleLiveUpdate(syncs []model.LiveUpdateSyncStep, runs []model.LiveUpdateRunStep, shouldRestart bool, fallBackOn []string, f pather) (model.LiveUpdate, error) {
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
		return model.LiveUpdate{}, err
	}
	return lu, nil
}

func imageTargetWithLiveUpdate(i model.ImageTarget, lu model.LiveUpdate) model.ImageTarget {
	db := i.DockerBuildInfo()
	db.LiveUpdate = lu
	return i.WithBuildDetails(db)
}
