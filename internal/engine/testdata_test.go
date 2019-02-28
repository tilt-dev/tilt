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

const SanchoStaticDockerfile = `
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

var SanchoRef = container.MustParseNamed("gcr.io/some-project-162817/sancho")
var SanchoBaseRef = container.MustParseNamed("sancho-base")
var SanchoSidecarRef = container.MustParseNamed("gcr.io/some-project-162817/sancho-sidecar")

func NewSanchoFastBuild(fixture pather) model.FastBuild {
	return model.FastBuild{
		BaseDockerfile: SanchoBaseDockerfile,
		Mounts: []model.Mount{
			model.Mount{
				LocalPath:     fixture.Path(),
				ContainerPath: "/go/src/github.com/windmilleng/sancho",
			},
		},
		Steps: model.ToSteps("/", []model.Cmd{
			model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}},
		}),
		Entrypoint: model.Cmd{Argv: []string{"/go/bin/sancho"}},
	}
}

func NewSanchoFastBuildManifest(fixture pather) model.Manifest {
	fbInfo := NewSanchoFastBuild(fixture)
	m := model.Manifest{
		Name: "sancho",
	}

	return assembleK8sManifest(
		m,
		model.K8sTarget{YAML: SanchoYAML},
		model.ImageTarget{Ref: SanchoRef}.WithBuildDetails(fbInfo))
}

func NewSanchoFastBuildManifestWithCache(fixture pather, paths []string) model.Manifest {
	manifest := NewSanchoFastBuildManifest(fixture)
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithCachePaths(paths))
	return manifest
}

func NewSanchoCustomBuildManifest(fixture pather) model.Manifest {
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
		model.ImageTarget{Ref: SanchoRef}.WithBuildDetails(cb))
}

func NewSanchoStaticImageTarget() model.ImageTarget {
	return model.ImageTarget{
		Ref: SanchoRef,
	}.WithBuildDetails(model.StaticBuild{
		Dockerfile: SanchoStaticDockerfile,
		BuildPath:  "/path/to/build",
	})
}

func NewSanchoSidecarStaticImageTarget() model.ImageTarget {
	iTarget := NewSanchoStaticImageTarget()
	iTarget.Ref = SanchoSidecarRef
	return iTarget
}

func NewSanchoStaticManifest() model.Manifest {
	return assembleK8sManifest(
		model.Manifest{Name: "sancho"},
		model.K8sTarget{YAML: SanchoYAML},
		NewSanchoStaticImageTarget().
			WithBuildDetails(model.StaticBuild{
				Dockerfile: SanchoStaticDockerfile,
				BuildPath:  "/path/to/build",
			}))
}

func NewSanchoStaticManifestWithCache(paths []string) model.Manifest {
	manifest := NewSanchoStaticManifest()
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithCachePaths(paths))
	return manifest
}

func NewSanchoStaticMultiStageManifest() model.Manifest {
	baseImage := model.ImageTarget{
		Ref: SanchoBaseRef,
	}.WithBuildDetails(model.StaticBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  "/path/to/build",
	})

	srcImage := model.ImageTarget{
		Ref: SanchoRef,
	}.WithBuildDetails(model.StaticBuild{
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
	baseImage := model.ImageTarget{
		Ref: SanchoBaseRef,
	}.WithBuildDetails(model.StaticBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  "/path/to/build",
	})

	fbInfo := NewSanchoFastBuild(fixture)
	fbInfo.BaseDockerfile = `FROM sancho-base`

	srcImage := model.ImageTarget{Ref: SanchoRef}.
		WithBuildDetails(fbInfo).
		WithDependencyIDs([]model.TargetID{baseImage.ID()})

	kTarget := model.K8sTarget{YAML: SanchoYAML}.
		WithDependencyIDs([]model.TargetID{srcImage.ID()})

	return model.Manifest{Name: "sancho"}.
		WithImageTargets([]model.ImageTarget{baseImage, srcImage}).
		WithDeployTarget(kTarget)
}

func NewManifestsWithCommonAncestor(fixture pather) (model.Manifest, model.Manifest) {
	refCommon := container.MustParseNamed("gcr.io/common")
	ref1 := container.MustParseNamed("gcr.io/image-1")
	ref2 := container.MustParseNamed("gcr.io/image-2")

	fixture.MkdirAll("common")
	fixture.MkdirAll("image-1")
	fixture.MkdirAll("image-2")

	targetCommon := model.ImageTarget{Ref: refCommon}.WithBuildDetails(model.StaticBuild{
		Dockerfile: `FROM golang:1.10`,
		BuildPath:  fixture.JoinPath("common"),
	})
	target1 := model.ImageTarget{Ref: ref1}.WithBuildDetails(model.StaticBuild{
		Dockerfile: `FROM ` + refCommon.String(),
		BuildPath:  fixture.JoinPath("image-1"),
	})
	target2 := model.ImageTarget{Ref: ref2}.WithBuildDetails(model.StaticBuild{
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
