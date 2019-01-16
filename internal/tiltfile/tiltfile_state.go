package tiltfile

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type resourceSet struct {
	dc  dcResource // currently only support one d-c.yml
	k8s []*k8sResource
}

type tiltfileState struct {
	// set at creation
	ctx      context.Context
	filename localPath

	// added to during execution
	configFiles    []string
	images         []*dockerImage
	imagesByName   map[string]*dockerImage
	k8s            []*k8sResource
	k8sByName      map[string]*k8sResource
	k8sUnresourced []k8s.K8sEntity
	dc             dcResource // currently only support one d-c.yml

	// for assembly
	usedImages map[string]bool

	builtinsMap starlark.StringDict
}

func newTiltfileState(ctx context.Context, filename string, tfRoot string) *tiltfileState {
	lp := localPath{path: filename}
	s := &tiltfileState{
		ctx:          ctx,
		filename:     localPath{path: filename},
		imagesByName: make(map[string]*dockerImage),
		k8sByName:    make(map[string]*k8sResource),
		configFiles:  []string{filename},
		usedImages:   make(map[string]bool),
	}
	s.filename = s.maybeAttachGitRepo(lp, filepath.Dir(lp.path))
	return s
}

func (s *tiltfileState) exec() error {
	thread := &starlark.Thread{
		Print: func(_ *starlark.Thread, msg string) {
			logger.Get(s.ctx).Infof("%s", msg)
		},
	}

	_, err := starlark.ExecFile(thread, s.filename.path, nil, s.builtins())
	return err
}

// Builtin functions

const (
	// build functions
	dockerComposeN = "docker_compose"
	dockerBuildN   = "docker_build"
	fastBuildN     = "fast_build"

	// k8s functions
	k8sYamlN     = "k8s_yaml"
	k8sResourceN = "k8s_resource"
	portForwardN = "port_forward"

	// file functions
	localGitRepoN = "local_git_repo"
	localN        = "local"
	readFileN     = "read_file"
	kustomizeN    = "kustomize"
	helmN         = "helm"
)

func (s *tiltfileState) builtins() starlark.StringDict {
	if s.builtinsMap != nil {
		return s.builtinsMap
	}

	addBuiltin := func(r starlark.StringDict, name string, fn func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)) {
		r[name] = starlark.NewBuiltin(name, fn)
	}

	r := make(starlark.StringDict)

	addBuiltin(r, localN, s.local)
	addBuiltin(r, readFileN, s.skylarkReadFile)

	addBuiltin(r, dockerComposeN, s.dockerCompose)
	addBuiltin(r, dockerBuildN, s.dockerBuild)
	addBuiltin(r, fastBuildN, s.fastBuild)
	addBuiltin(r, k8sYamlN, s.k8sYaml)
	addBuiltin(r, k8sResourceN, s.k8sResource)
	addBuiltin(r, portForwardN, s.portForward)
	addBuiltin(r, localGitRepoN, s.localGitRepo)
	addBuiltin(r, kustomizeN, s.kustomize)
	addBuiltin(r, helmN, s.helm)

	s.builtinsMap = r

	return r
}

func (s *tiltfileState) assemble() (resourceSet, []k8s.K8sEntity, error) {
	images, err := s.findUnresourcedImages()
	if err != nil {
		return resourceSet{}, nil, err
	}
	for _, image := range images {
		if _, ok := s.imagesByName[image.Name()]; !ok {
			// only expand for images we know how to build
			continue
		}
		target, err := s.findExpandTarget(image)
		if err != nil {
			return resourceSet{}, nil, err
		}
		if err := s.extractImage(target, image); err != nil {
			return resourceSet{}, nil, err
		}
	}

	assembledImages := map[string]bool{}
	for _, r := range s.k8s {
		if err := s.validateK8s(r, assembledImages); err != nil {
			return resourceSet{}, nil, err
		}
	}

	for k, _ := range s.imagesByName {
		if !assembledImages[k] {
			return resourceSet{}, nil, fmt.Errorf("image %v is not used in any resource", k)
		}
	}

	if !s.dc.Empty() && len(s.k8s) > 0 {
		return resourceSet{}, nil, fmt.Errorf("can't declare both k8s resources and docker-compose resources")
	}

	return resourceSet{
		dc:  s.dc,
		k8s: s.k8s,
	}, s.k8sUnresourced, nil
}

func (s *tiltfileState) validateK8s(r *k8sResource, assembledImages map[string]bool) error {
	var images []reference.Named
	for _, e := range r.k8s {
		entityImages, err := e.FindImages()
		if err != nil {
			return err
		}
		images = append(images, entityImages...)
	}

	for _, image := range images {
		if _, ok := s.imagesByName[image.Name()]; !ok {
			continue
		}
		if r.imageRef == "" {
			r.imageRef = image.Name()
		} else if r.imageRef == image.Name() {
			continue
		} else {
			return fmt.Errorf("resource %q contains two images to be built: %q, %q. You can use k8s_yaml to include a lot of yaml and then Tilt will create resources automatically. If this doesn't solve your case (e.g. you have one pod that has two images that Tilt updates), please reach out so we can understand and prioritize", r.name, r.imageRef, image.Name())
		}
	}

	if len(r.k8s) == 0 {
		if r.imageRef == "" {
			return fmt.Errorf("resource %q: no matching resource", r.name)
		}
		return fmt.Errorf("resource %q: could not find image %q; perhaps there's a typo?", r.name, r.imageRef)
	}

	assembledImages[r.imageRef] = true

	return nil
}

func (s *tiltfileState) findExpandTarget(image reference.Named) (*k8sResource, error) {
	// first, match an empty resource that has this exact imageRef
	for _, r := range s.k8s {
		if len(r.k8s) == 0 && r.imageRef == image.Name() {
			return r, nil
		}
	}

	// next, match an empty resource that has the same name
	name := filepath.Base(image.Name())
	for _, r := range s.k8s {
		if len(r.k8s) == 0 && r.name == name {
			return r, nil
		}
	}

	// otherwise, create a new resource
	return s.makeK8sResource(name)
}

func (s *tiltfileState) findUnresourcedImages() ([]reference.Named, error) {
	var result []reference.Named
	seen := make(map[string]bool)

	for _, e := range s.k8sUnresourced {
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
			return nil, fmt.Errorf("Found an entity with multiple images registered with k8s_yaml. Tilt doesn't support this yet; please reach out so we can understand and prioritize this case. found images: %q, entity: %q.", entityImages, str)
		}
		img := entityImages[0]
		if !seen[img.Name()] {
			result = append(result, img)
			seen[img.Name()] = true
		}
	}
	return result, nil
}

func (s *tiltfileState) extractImage(dest *k8sResource, imageRef reference.Named) error {
	extracted, remaining, err := k8s.FilterByImage(s.k8sUnresourced, imageRef)
	if err != nil {
		return err
	}

	dest.k8s = append(dest.k8s, extracted...)
	s.k8sUnresourced = remaining

	for _, e := range extracted {
		podTemplates, err := k8s.ExtractPodTemplateSpec(e)
		if err != nil {
			return err
		}
		for _, template := range podTemplates {
			extracted, remaining, err := k8s.FilterByLabels(s.k8sUnresourced, template.Labels)
			if err != nil {
				return err
			}
			dest.k8s = append(dest.k8s, extracted...)
			s.k8sUnresourced = remaining
		}
	}
	return nil
}

// If the user requested only a subset of manifests, filter those manifests out.
func match(manifests []model.Manifest, matching map[string]bool) ([]model.Manifest, error) {
	if len(matching) == 0 {
		return manifests, nil
	}

	var result []model.Manifest
	matched := make(map[string]bool, len(matching))
	var unmatchedNames []string
	for _, m := range manifests {
		if !matching[string(m.Name)] {
			unmatchedNames = append(unmatchedNames, m.Name.String())
			continue
		}
		result = append(result, m)
		matched[string(m.Name)] = true
	}

	if len(matched) != len(matching) {
		var missing []string
		for k := range matching {
			if !matched[k] {
				missing = append(missing, k)
			}
		}

		return nil, fmt.Errorf("Could not find resources: %s. Existing resources in Tiltfile: %s",
			strings.Join(missing, ", "), strings.Join(unmatchedNames, ", "))
	}

	return result, nil
}

func (s *tiltfileState) translateK8s(resources []*k8sResource) ([]model.Manifest, error) {
	var result []model.Manifest
	for _, r := range resources {
		m := model.Manifest{
			Name: model.ManifestName(r.name),
		}

		k8sYaml, err := k8s.SerializeYAML(r.k8s)
		if err != nil {
			return nil, err
		}

		m = m.WithDeployTarget(model.K8sTarget{
			YAML:         k8sYaml,
			PortForwards: s.portForwardsToDomain(r), // FIXME(dbentley)
		})

		if r.imageRef != "" {
			image := s.imagesByName[r.imageRef]
			isStaticBuild := !image.staticBuildPath.Empty()
			isFastBuild := !image.baseDockerfilePath.Empty()

			dInfo := model.ImageTarget{
				Ref: image.ref,
			}.WithCachePaths(image.cachePaths)

			if isStaticBuild && isFastBuild {
				return nil, fmt.Errorf("cannot populate both staticBuild and fastBuild properties")
			} else if isStaticBuild {
				dInfo = dInfo.WithBuildDetails(model.StaticBuild{
					Dockerfile: image.staticDockerfile.String(),
					BuildPath:  string(image.staticBuildPath.path),
					BuildArgs:  image.staticBuildArgs,
				})
			} else if isFastBuild {
				dInfo = dInfo.WithBuildDetails(model.FastBuild{
					BaseDockerfile: image.baseDockerfile.String(),
					Mounts:         s.mountsToDomain(image),
					Steps:          image.steps,
					Entrypoint:     model.ToShellCmd(image.entrypoint),
				})
			} else {
				return nil, fmt.Errorf("internal Tilt error: no build info for manifest %s", r.name)
			}

			m = m.WithImageTarget(dInfo.
				WithRepos(s.reposForImage(image)).
				WithDockerignores(s.dockerignoresForImage(image)).
				WithTiltFilename(s.filename.path))
			m = m.WithTiltFilename(s.filename.path)
		}
		result = append(result, m)
	}

	return result, nil
}

func (s *tiltfileState) translateDC(dc dcResource) ([]model.Manifest, error) {
	var result []model.Manifest
	for _, svc := range dc.services {
		m, configFiles, err := s.dcServiceToManifest(svc, dc.configPath)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
		s.configFiles = sliceutils.DedupeStringSlice(append(s.configFiles, configFiles...))
	}
	if dc.configPath != "" {
		s.configFiles = sliceutils.DedupeStringSlice(append(s.configFiles, dc.configPath))
	}
	return result, nil
}

func badTypeErr(b *starlark.Builtin, ex interface{}, v starlark.Value) error {
	return fmt.Errorf("%v expects a %T; got %T (%v)", b.Name(), ex, v, v)
}
