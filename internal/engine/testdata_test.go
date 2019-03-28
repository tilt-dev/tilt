package engine

import (
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

var SanchoRef = container.MustParseSelector("gcr.io/some-project-162817/sancho")
var SanchoBaseRef = container.MustParseSelector("sancho-base")
var SanchoSidecarRef = container.MustParseSelector("gcr.io/some-project-162817/sancho-sidecar")

func NewSanchoFastBuild(fixture pather) model.FastBuild {
	return model.FastBuild{
		BaseDockerfile: SanchoBaseDockerfile,
		Mounts: []model.Mount{
			model.Mount{
				LocalPath:     fixture.Path(),
				ContainerPath: "/go/src/github.com/windmilleng/sancho",
			},
		},
		Runs: model.ToRuns("/", []model.Cmd{
			model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}},
		}),
		Entrypoint: model.Cmd{Argv: []string{"/go/bin/sancho"}},
	}
}
func SanchoSyncSteps(fixture pather) []model.LiveUpdateSyncStep {
	return []model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{fixture.Path(), "/go/src/github.com/windmilleng/sancho"}}
}

var SanchoRunSteps = []model.LiveUpdateRunStep{model.LiveUpdateRunStep{Command: model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}}}}

func NewSanchoLiveUpdate(fixture pather) model.LiveUpdate {
	steps := []model.LiveUpdateStep{
		model.LiveUpdateSyncStep{fixture.Path(), "/go/src/github.com/windmilleng/sancho"},
		model.LiveUpdateRunStep{Command: model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}}},
		model.LiveUpdateRestartContainerStep{},
	}
	return model.MustNewLiveUpdate(steps, nil)
}

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
		Fast:    &fb,
	}

	m := model.Manifest{Name: "sancho"}

	return assembleK8sManifest(
		m,
		model.K8sTarget{YAML: SanchoYAML},
		model.NewImageTarget(SanchoRef).WithBuildDetails(cb))
}

func NewSanchoCustomBuildManifestWithLiveUpdate(fixture pather) model.Manifest {
	lu := NewSanchoLiveUpdate(fixture)
	cb := model.CustomBuild{
		Command:    "true",
		Deps:       []string{fixture.JoinPath("app")},
		LiveUpdate: &lu,
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

func NewSanchoDockerBuildImageTarget() model.ImageTarget {
	return model.NewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  "/path/to/build",
	})
}

func NewSanchoSidecarDockerBuildImageTarget() model.ImageTarget {
	iTarget := NewSanchoDockerBuildImageTarget()
	iTarget.ConfigurationRef = SanchoSidecarRef
	iTarget.DeploymentRef = SanchoSidecarRef.AsNamedOnly()
	return iTarget
}

func NewSanchoDockerBuildManifest() model.Manifest {
	return assembleK8sManifest(
		model.Manifest{Name: "sancho"},
		model.K8sTarget{YAML: SanchoYAML},
		NewSanchoDockerBuildImageTarget().
			WithBuildDetails(model.DockerBuild{
				Dockerfile: SanchoDockerfile,
				BuildPath:  "/path/to/build",
			}))
}

func NewSanchoDockerBuildManifestWithCache(paths []string) model.Manifest {
	manifest := NewSanchoDockerBuildManifest()
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithCachePaths(paths))
	return manifest
}

func NewSanchoDockerBuildManifestWithNestedFastBuild(fixture pather) model.Manifest {
	manifest := NewSanchoDockerBuildManifest()
	iTarg := manifest.ImageTargetAt(0)
	fb := NewSanchoFastBuild(fixture)
	sb := iTarg.DockerBuildInfo()
	sb.FastBuild = &fb
	iTarg = iTarg.WithBuildDetails(sb)
	manifest = manifest.WithImageTarget(iTarg)
	return manifest
}

func NewSanchoDockerBuildManifestWithLiveUpdate(fixture pather) model.Manifest {
	manifest := NewSanchoDockerBuildManifest()
	iTarg := manifest.ImageTargetAt(0)
	lu := NewSanchoLiveUpdate(fixture)
	db := iTarg.DockerBuildInfo()
	db.LiveUpdate = &lu
	iTarg = iTarg.WithBuildDetails(db)
	manifest = manifest.WithImageTarget(iTarg)
	return manifest
}

func NewSanchoDockerBuildMultiStageManifest() model.Manifest {
	baseImage := model.NewImageTarget(SanchoBaseRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  "/path/to/build",
	})

	srcImage := model.NewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `
FROM sancho-base
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
		BuildPath: "/path/to/build",
	}).WithDependencyIDs([]model.TargetID{baseImage.ID()})

	kTarget := model.K8sTarget{YAML: SanchoYAML}.
		WithDependencyIDs([]model.TargetID{srcImage.ID()})

	return model.Manifest{Name: "sancho"}.
		WithImageTargets([]model.ImageTarget{baseImage, srcImage}).
		WithDeployTarget(kTarget)
}

func NewSanchoFastMultiStageManifest(fixture pather) model.Manifest {
	baseImage := model.NewImageTarget(SanchoBaseRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  "/path/to/build",
	})

	fbInfo := NewSanchoFastBuild(fixture)
	fbInfo.BaseDockerfile = `FROM sancho-base`

	srcImage := model.NewImageTarget(SanchoRef).
		WithBuildDetails(fbInfo).
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
