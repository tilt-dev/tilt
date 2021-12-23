package build

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Derived from
// https://github.com/moby/buildkit/blob/175e8415e38228dbb75e6b54efd2c8e9fc5b1cbf/util/archutil/detect.go#L15
var validBuildkitArchSet = map[string]bool{
	"amd64":    true,
	"arm64":    true,
	"riscv64":  true,
	"ppc64le":  true,
	"s390x":    true,
	"386":      true,
	"mips64le": true,
	"mips64":   true,
	"arm":      true,
}

// Create a new ImageTarget with the platform OS/Arch from the target cluster.
func InjectClusterPlatform(spec v1alpha1.DockerImageSpec, cluster *v1alpha1.Cluster) v1alpha1.DockerImageSpec {
	if spec.Platform != "" || cluster == nil {
		return spec
	}

	// Eventually, it might make sense to read the supported platforms
	// off the buildkit server and negotiate the right one, but for
	// now we hard-code a whitelist.
	targetArch := cluster.Status.Arch
	if !validBuildkitArchSet[targetArch] {
		return spec
	}

	if targetArch == "arm" {
		// This is typically communicated with GOARM.
		// For now, just assume arm/v7
		targetArch = "arm/v7"
	}

	// Currently Tilt only supports linux containers.
	// We don't even build windows-compatible docker contexts.
	spec.Platform = fmt.Sprintf("linux/%s", targetArch)
	return spec
}

// Create a new ImageTarget with the Dockerfiles rewritten with the injected images.
func InjectImageDependencies(spec v1alpha1.DockerImageSpec, imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) (v1alpha1.DockerImageSpec, error) {
	if len(spec.ImageMaps) == 0 {
		return spec, nil
	}

	df := dockerfile.Dockerfile(spec.DockerfileContents)
	buildArgs := spec.Args

	ast, err := dockerfile.ParseAST(df)
	if err != nil {
		return spec, errors.Wrap(err, "injectImageDependencies")
	}

	for _, dep := range spec.ImageMaps {
		im, ok := imageMaps[types.NamespacedName{Name: dep}]
		if !ok || im.Status.ImageFromLocal == "" {
			return spec, fmt.Errorf("missing image dependency: %s", dep)
		}

		image := im.Status.ImageFromLocal
		imageRef, err := container.ParseNamedTagged(image)
		if err != nil {
			return spec, errors.Wrap(err, "injectImageDependencies parse")
		}

		selector, err := container.SelectorFromImageMap(im.Spec)
		if err != nil {
			return spec, errors.Wrap(err, "injectImageDependencies selector")
		}

		modified, err := ast.InjectImageDigest(selector, imageRef, buildArgs)
		if err != nil {
			return spec, errors.Wrap(err, "injectImageDependencies inject")
		} else if !modified {
			return spec, fmt.Errorf("Could not inject image %q into Dockerfile of image %q", image, selector)
		}
	}

	newDf, err := ast.Print()
	if err != nil {
		return spec, errors.Wrap(err, "injectImageDependencies")
	}

	spec.DockerfileContents = newDf.String()

	return spec, nil
}
