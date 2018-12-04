package tiltfile2

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/docker/distribution/reference"
	"github.com/google/skylark"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type tiltfileState struct {
	// set at creation
	ctx      context.Context
	filename string

	// added to during execution
	configFiles  []string
	images       []*dockerImage
	imagesByName map[string]*dockerImage
	k8s          []*k8sResource
	k8sByName    map[string]*k8sResource
}

func newTiltfileState(ctx context.Context, filename string) *tiltfileState {
	return &tiltfileState{
		ctx:          ctx,
		filename:     filename,
		imagesByName: make(map[string]*dockerImage),
		k8sByName:    make(map[string]*k8sResource),
		configFiles:  []string{filename},
	}
}

func (s *tiltfileState) exec() error {
	thread := &skylark.Thread{
		Print: func(_ *skylark.Thread, msg string) {
			logger.Get(s.ctx).Infof("%s", msg)
		},
	}
	_, err := skylark.ExecFile(thread, s.filename, nil, s.builtins())
	return err
}

// Builtin functions

const (
	// build functions
	dockerBuildN = "docker_build"
	fastBuildN   = "fast_build"

	// k8s functions
	k8sResourceN = "k8s_resource"
	portForwardN = "port_forward"

	// file functions
	localGitRepoN = "local_git_repo"
	localN        = "local"
	readFileN     = "read_file"
	kustomizeN    = "kustomize"
)

func (s *tiltfileState) builtins() skylark.StringDict {
	r := make(skylark.StringDict)
	add := func(name string, fn func(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error)) {
		r[name] = skylark.NewBuiltin(name, fn)
	}

	add(dockerBuildN, s.dockerBuild)
	add(fastBuildN, s.fastBuild)

	add(k8sResourceN, s.k8sResource)
	add(portForwardN, s.portForward)

	add(localGitRepoN, s.localGitRepo)
	add(localN, s.local)
	add(readFileN, s.skylarkReadFile)
	add(kustomizeN, s.kustomize)
	return r
}

func (s *tiltfileState) assemble() ([]*k8sResource, error) {
	var result []*k8sResource
	byImage := make(map[string]*k8sResource)

	for _, r := range s.k8s {
		result = append(result, r)
		if r.imageRef == "" {
			continue
		}
		if _, ok := s.imagesByName[r.imageRef]; !ok {
			var images []string
			for _, img := range s.images {
				images = append(images, img.ref.Name())
			}
			return nil, fmt.Errorf("Resource %q requires image %q, but it is not configured (available images are %q)", r.name, r.imageRef, images)
		}
		byImage[r.imageRef] = r
	}

	// For resources that have imageRef but no k8s, extract from other resources
	for _, r := range result {
		if len(r.k8s) > 0 || r.imageRef == "" {
			continue
		}

		ref, err := reference.ParseNormalizedNamed(r.imageRef)
		if err != nil {
			return nil, err
		}
		for _, source := range result {
			if source == r {
				continue
			}
			if err := s.extractImage(r, source, ref); err != nil {
				return nil, err
			}
		}
	}

	// Expand resources with multiple images
	for _, r := range result {
		resourceImages, err := s.findImages(r)
		if err != nil {
			return nil, err
		}

		if len(resourceImages) < 2 {
			if len(resourceImages) == 1 {
				r.imageRef = resourceImages[0].Name()
			}
			continue
		}

		for _, ref := range resourceImages {
			target := byImage[ref.Name()]
			if target == nil {
				target = &k8sResource{
					expandedFrom: r.name,
					imageRef:     ref.Name(),
				}
				byImage[ref.Name()] = target
				result = append(result, target)
			}
			if err := s.extractImage(target, r, ref); err != nil {
				return nil, err
			}
		}
	}

	return s.assignNames(result)
}

func (s *tiltfileState) findImages(r *k8sResource) ([]reference.Named, error) {
	var result []reference.Named

	for _, e := range r.k8s {
		images, err := e.FindImages()
		if err != nil {
			return nil, err
		}
		var entityImages []reference.Named
		for _, image := range images {
			if _, ok := s.imagesByName[image.Name()]; ok {
				entityImages = append(entityImages, image)
			}
		}
		if len(entityImages) == 0 {
			continue
		}
		if len(entityImages) > 1 {
			str, err := k8s.SerializeYAML([]k8s.K8sEntity{e})
			if err != nil {
				str = err.Error()
			}
			return nil, fmt.Errorf("Found an entity with multiple images registered with Tilt. Tilt doesn't support this yet; please reach out so we can understand and prioritize this case. Resource: %q, found images: %q, entity: %q.", r.name, entityImages, str)
		}
		result = append(result, entityImages[0])
	}
	return result, nil
}

func (s *tiltfileState) extractImage(dest, src *k8sResource, imageRef reference.Named) error {
	extracted, remaining, err := k8s.FilterByImage(src.k8s, imageRef)
	if err != nil {
		return err
	}

	dest.k8s = append(dest.k8s, extracted...)
	src.k8s = remaining

	for _, e := range extracted {
		podTemplates, err := k8s.ExtractPodTemplateSpec(e)
		if err != nil {
			return err
		}
		for _, template := range podTemplates {
			extracted, remaining, err := k8s.FilterByLabels(src.k8s, template.Labels)
			if err != nil {
				return err
			}
			dest.k8s = append(dest.k8s, extracted...)
			src.k8s = remaining
		}
	}
	return nil
}

func (s *tiltfileState) assignNames(result []*k8sResource) ([]*k8sResource, error) {
	byName := make(map[string]int)
	byImage := make(map[string][]string)

	for _, r := range result {
		if r.name != "" {
			// declared name
			byName[r.name]++
			continue
		}

		if r.imageRef == "" {
			return nil, fmt.Errorf("resource has neither name nor image ref; this should be impossible please report to Tilt team")
		}

		p := r.imageRef
		n := ""
		var names []string
		// We want to create as short a name as possible that doesn't conflict.
		// If it's gcr.io/company/project/foo, we'll try, in order:
		//  foo
		//  project/foo
		//  company/project/foo
		//  gcr.io/company/project/foo
		for p != "" && p != "." {
			var base string
			p, base = filepath.Dir(p), filepath.Base(p)
			n = filepath.Join(base, n)
			names = append(names, n)
			byName[n]++
		}
		byImage[r.imageRef] = names
	}

	// Now check which is free
	for _, r := range result {
		if r.name != "" {
			continue
		}
		candidates := byImage[r.imageRef]
		for _, candidate := range candidates {
			if byName[candidate] == 1 {
				r.name = candidate
				break
			}
		}
	}

	return result, nil
}

func (s *tiltfileState) translate(resources []*k8sResource) ([]model.Manifest, error) {
	var result []model.Manifest
	for _, r := range resources {
		m := model.Manifest{
			Name: model.ManifestName(r.name),
		}

		k8sYaml, err := k8s.SerializeYAML(r.k8s)
		if err != nil {
			return nil, err
		}

		m = m.WithPortForwards(s.portForwardsToDomain(r)). // FIXME(dbentley)
									WithK8sYAML(k8sYaml)

		if r.imageRef != "" {
			image := s.imagesByName[r.imageRef]
			m.Mounts = s.mountsToDomain(image)
			m.Entrypoint = model.ToShellCmd(image.entrypoint)
			m.BaseDockerfile = image.baseDockerfile.String()
			m.Steps = image.steps
			m.StaticDockerfile = image.staticDockerfile.String()
			m.StaticBuildPath = string(image.staticBuildPath.path)
			m.StaticBuildArgs = image.staticBuildArgs
			m.Repos = s.reposToDomain(image)
			m = m.WithDockerRef(image.ref).
				WithTiltFilename(image.tiltFilename).
				WithCachePaths(image.cachePaths)
		}
		result = append(result, m)
	}

	return result, nil
}

func badTypeErr(b *skylark.Builtin, ex interface{}, v skylark.Value) error {
	return fmt.Errorf("%v expects a %T; got %T (%v)", b.Name(), ex, v, v)
}
