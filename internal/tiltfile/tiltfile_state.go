package tiltfile

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

type resourceSet struct {
	dc  dcResourceSet // currently only support one d-c.yml
	k8s []*k8sResource
}

type tiltfileState struct {
	// set at creation
	ctx      context.Context
	filename localPath
	dcCli    dockercompose.DockerComposeClient

	// added to during execution
	configFiles    []string
	buildIndex     *buildIndex
	k8s            []*k8sResource
	k8sByName      map[string]*k8sResource
	k8sUnresourced []k8s.K8sEntity
	dc             dcResourceSet // currently only support one d-c.yml

	// for assembly
	usedImages map[string]bool

	builtinsMap starlark.StringDict

	logger   *log.Logger
	warnings []string
}

func newTiltfileState(ctx context.Context, filename string, tfRoot string, l *log.Logger) *tiltfileState {
	lp := localPath{path: filename}
	s := &tiltfileState{
		ctx:         ctx,
		filename:    localPath{path: filename},
		dcCli:       dockercompose.NewDockerComposeClient(),
		buildIndex:  newBuildIndex(),
		k8sByName:   make(map[string]*k8sResource),
		configFiles: []string{filename},
		usedImages:  make(map[string]bool),
		logger:      l,
	}
	s.filename = s.maybeAttachGitRepo(lp, filepath.Dir(lp.path))
	return s
}

func (s *tiltfileState) exec() error {
	thread := &starlark.Thread{
		Print: func(_ *starlark.Thread, msg string) {
			s.logger.Printf("%s", msg)
		},
	}

	s.logger.Printf("Beginning Tiltfile execution")
	_, err := starlark.ExecFile(thread, s.filename.path, nil, s.builtins())
	if err == nil {
		s.logger.Printf("Successfully executed Tiltfile")
	}
	return err
}

// Builtin functions

const (
	// build functions
	dockerBuildN = "docker_build"
	fastBuildN   = "fast_build"
	customBuildN = "customBuild"

	// docker compose functions
	dockerComposeN = "docker_compose"
	dcResourceN    = "dc_resource"

	// k8s functions
	k8sYamlN     = "k8s_yaml"
	filterYamlN  = "filter_yaml"
	k8sResourceN = "k8s_resource"
	portForwardN = "port_forward"

	// file functions
	localGitRepoN = "local_git_repo"
	localN        = "local"
	readFileN     = "read_file"
	kustomizeN    = "kustomize"
	helmN         = "helm"
	listdirN      = "listdir"

	// other functions
	failN = "fail"
	blobN = "blob"
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

	addBuiltin(r, dockerBuildN, s.dockerBuild)
	addBuiltin(r, fastBuildN, s.fastBuild)
	addBuiltin(r, customBuildN, s.customBuild)
	addBuiltin(r, dockerComposeN, s.dockerCompose)
	addBuiltin(r, dcResourceN, s.dcResource)
	addBuiltin(r, k8sYamlN, s.k8sYaml)
	addBuiltin(r, filterYamlN, s.filterYaml)
	addBuiltin(r, k8sResourceN, s.k8sResource)
	addBuiltin(r, portForwardN, s.portForward)
	addBuiltin(r, localGitRepoN, s.localGitRepo)
	addBuiltin(r, kustomizeN, s.kustomize)
	addBuiltin(r, helmN, s.helm)
	addBuiltin(r, failN, s.fail)
	addBuiltin(r, blobN, s.blob)
	addBuiltin(r, listdirN, s.listdir)

	s.builtinsMap = r

	return r
}

func (s *tiltfileState) assemble() (resourceSet, []k8s.K8sEntity, error) {
	err := s.assembleImages()
	if err != nil {
		return resourceSet{}, nil, err
	}

	err = s.assembleK8s()
	if err != nil {
		return resourceSet{}, nil, err
	}

	err = s.assembleDC()
	if err != nil {
		return resourceSet{}, nil, err
	}

	if !s.dc.Empty() && (len(s.k8s) > 0 || len(s.k8sUnresourced) > 0) {
		return resourceSet{}, nil, fmt.Errorf("can't declare both k8s " +
			"resources/entities and docker-compose resources")
	}

	err = s.buildIndex.assertAllMatched()
	if err != nil {
		s.logger.Printf("WARNING: %s\n", err)
		s.warnings = append(s.warnings, err.Error())
	}

	return resourceSet{
		dc:  s.dc,
		k8s: s.k8s,
	}, s.k8sUnresourced, nil
}

func (s *tiltfileState) assembleImages() error {
	for _, imageBuilder := range s.buildIndex.images {
		var depImages []reference.Named
		var err error
		if imageBuilder.baseDockerfile != "" {
			depImages, err = imageBuilder.baseDockerfile.FindImages()
		} else {
			depImages, err = imageBuilder.staticDockerfile.FindImages()
		}

		if err != nil {
			return err
		}

		for _, depImage := range depImages {
			depBuilder := s.buildIndex.findBuilderForConsumedImage(depImage)
			if depBuilder != nil {
				imageBuilder.dependencyIDs = append(imageBuilder.dependencyIDs, depBuilder.ID())
			}
		}
	}
	return nil
}

func (s *tiltfileState) assembleDC() error {
	for _, svc := range s.dc.services {
		if svc.ImageRef != nil {
			builder := s.buildIndex.findBuilderForConsumedImage(svc.ImageRef)
			if builder != nil {
				svc.DependencyIDs = append(svc.DependencyIDs, builder.ID())
			}
		}
	}
	return nil
}

func (s *tiltfileState) assembleK8s() error {
	err := s.assembleK8sWithImages()
	if err != nil {
		return err
	}

	err = s.assembleK8sUnresourced()
	if err != nil {
		return err
	}

	for _, r := range s.k8s {
		if err := s.validateK8s(r); err != nil {
			return err
		}
	}
	return nil

}

// assembleK8sWithImages matches images we know how to build with any k8s entities
// that use that image, storing the resulting resource(s) on the tiltfileState.
func (s *tiltfileState) assembleK8sWithImages() error {
	// find all images mentioned in k8s entities that don't yet belong to k8sResources
	k8sRefs, err := s.findUnresourcedImages()
	if err != nil {
		return err
	}
	for _, k8sRef := range k8sRefs {
		image := s.buildIndex.findBuilderForConsumedImage(k8sRef)
		if image == nil {
			// only expand for images we know how to build
			continue
		}

		ref := image.ref
		target, err := s.k8sResourceForImage(ref)
		if err != nil {
			return err
		}
		// find k8s entities that use this image; pull them out of pool of
		// unresourced entities and instead attach them to the target k8sResource
		if err := s.extractEntities(target, ref); err != nil {
			return err
		}
	}
	return nil
}

// assembleK8sUnresourced makes k8sResources for all unresourced k8s entities that will
// result in pods (smartly grouping pod-creating entities with corresponding entities e.g.
// services), and stores the resulting resource(s) on the tiltfileState.
func (s *tiltfileState) assembleK8sUnresourced() error {
	withPodSpec, allRest, err := k8s.FilterByHasPodTemplateSpec(s.k8sUnresourced)
	if err != nil {
		return nil
	}
	for _, e := range withPodSpec {
		target, err := s.k8sResourceForName(e.Name())
		if err != nil {
			return err
		}
		target.entities = append(target.entities, e)

		match, rest, err := k8s.FilterByMatchesPodTemplateSpec(e, allRest)
		target.entities = append(target.entities, match...)
		allRest = rest
	}

	s.k8sUnresourced = allRest

	return nil
}

func (s *tiltfileState) validateK8s(r *k8sResource) error {
	if len(r.entities) == 0 {
		if len(r.providedImageRefNames) > 0 {
			return fmt.Errorf("resource %q: could not find k8s entities matching "+
				"image(s) %q; perhaps there's a typo?",
				r.name, strings.Join(r.providedImageRefNameList(), "; "))
		}
		return fmt.Errorf("resource %q: could not associate any k8s YAML with this resource", r.name)
	}

	for _, ref := range r.imageRefs {
		builder := s.buildIndex.findBuilderForConsumedImage(ref)
		if builder != nil {
			r.dependencyIDs = append(r.dependencyIDs, builder.ID())
		}
	}

	return nil
}

// k8sResourceForImage returns the k8sResource with which this image is associated
// (either an existing resource or a new one).
func (s *tiltfileState) k8sResourceForImage(image reference.Named) (*k8sResource, error) {
	// first, look thru all the resources that have already been created,
	// and see if any of them already have a reference to this image.
	for _, r := range s.k8s {
		if _, ok := r.imageRefNames[image.Name()]; ok {
			return r, nil
		}
	}

	// next, look thru all the resources that have already been created,
	// and see if any of them match the basename of the image.
	name := filepath.Base(image.Name())
	if r, ok := s.k8sByName[name]; ok {
		return r, nil
	}

	// otherwise, create a new resource
	return s.makeK8sResource(name)
}

// k8sResourceForName returns the k8sResource with which this name is associated
// (either an existing resource or a new one).
func (s *tiltfileState) k8sResourceForName(name string) (*k8sResource, error) {
	if r, ok := s.k8sByName[name]; ok {
		return r, nil
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
		for _, img := range images {
			if !seen[img.Name()] {
				result = append(result, img)
				seen[img.Name()] = true
			}
		}
	}
	return result, nil
}

// extractEntities extracts k8s entities matching the image ref and stores them on the dest k8sResource
func (s *tiltfileState) extractEntities(dest *k8sResource, imageRef reference.Named) error {
	extracted, remaining, err := k8s.FilterByImage(s.k8sUnresourced, imageRef)
	if err != nil {
		return err
	}

	err = dest.addEntities(extracted)
	if err != nil {
		return err
	}

	s.k8sUnresourced = remaining

	for _, e := range extracted {
		match, rest, err := k8s.FilterByMatchesPodTemplateSpec(e, s.k8sUnresourced)
		if err != nil {
			return err
		}

		err = dest.addEntities(match)
		if err != nil {
			return err
		}
		s.k8sUnresourced = rest
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
		mn := model.ManifestName(r.name)
		m := model.Manifest{
			Name: mn,
		}

		k8sTarget, err := k8s.NewTarget(mn.TargetName(), r.entities, s.portForwardsToDomain(r), r.extraPodSelectors, r.dependencyIDs)
		if err != nil {
			return nil, err
		}

		m = m.WithDeployTarget(k8sTarget)

		iTargets, err := s.imgTargetsForDependencyIDs(r.dependencyIDs)
		if err != nil {
			return nil, errors.Wrapf(err, "getting image build info for %s", r.name)
		}

		m = m.WithImageTargets(iTargets)

		result = append(result, m)
	}

	return result, nil
}

// Grabs all image targets for the given references,
// as well as any of their transitive dependencies.
func (s *tiltfileState) imgTargetsForDependencyIDs(ids []model.TargetID) ([]model.ImageTarget, error) {
	claimStatus := make(map[model.TargetID]claim, len(ids))
	return s.imgTargetsForDependencyIDsHelper(ids, claimStatus)
}

func (s *tiltfileState) imgTargetsForDependencyIDsHelper(ids []model.TargetID, claimStatus map[model.TargetID]claim) ([]model.ImageTarget, error) {
	iTargets := make([]model.ImageTarget, 0, len(ids))
	for _, id := range ids {
		image := s.buildIndex.findBuilderByID(id)
		if image == nil {
			return nil, fmt.Errorf("Internal error: no image builder found for id %s", id)
		}

		claim := claimStatus[id]
		if claim == claimFinished {
			// Skip this target, an earlier call has already built it
			continue
		} else if claim == claimPending {
			return nil, fmt.Errorf("Image dependency cycle: %s", image.ref)
		}
		claimStatus[id] = claimPending

		iTarget := model.ImageTarget{
			Ref: image.ref,
		}.WithCachePaths(image.cachePaths)

		switch {
		case !image.staticBuildPath.Empty():
			iTarget = iTarget.WithBuildDetails(model.StaticBuild{
				Dockerfile: image.staticDockerfile.String(),
				BuildPath:  string(image.staticBuildPath.path),
				BuildArgs:  image.staticBuildArgs,
			})
		case !image.baseDockerfilePath.Empty():
			iTarget = iTarget.WithBuildDetails(model.FastBuild{
				BaseDockerfile: image.baseDockerfile.String(),
				Mounts:         s.mountsToDomain(image),
				Steps:          image.steps,
				Entrypoint:     model.ToShellCmd(image.entrypoint),
				HotReload:      image.hotReload,
			})
		case image.customCommand != "":
			r := model.CustomBuild{
				Command: image.customCommand,
				Deps:    image.customDeps,
			}
			if len(image.mounts) > 0 || len(image.steps) > 0 {
				r.Fast = &model.FastBuild{
					Mounts:    s.mountsToDomain(image),
					Steps:     image.steps,
					HotReload: image.hotReload,
				}
			}
			iTarget = iTarget.WithBuildDetails(r)
			// TODO(dbentley): validate that mounts is a subset of deps
		default:
			return nil, fmt.Errorf("no build info for image %s", image.ref)
		}

		iTarget = iTarget.
			WithRepos(s.reposForImage(image)).
			WithDockerignores(s.dockerignoresForImage(image)).
			WithTiltFilename(s.filename.path)

		depTargets, err := s.imgTargetsForDependencyIDsHelper(image.dependencyIDs, claimStatus)
		if err != nil {
			return nil, err
		}

		iTargets = append(iTargets, depTargets...)
		iTargets = append(iTargets, iTarget)

		claimStatus[id] = claimFinished
	}
	return iTargets, nil
}

func (s *tiltfileState) translateDC(dc dcResourceSet) ([]model.Manifest, error) {
	var result []model.Manifest
	for _, svc := range dc.services {
		m, configFiles, err := s.dcServiceToManifest(svc, dc.configPath)
		if err != nil {
			return nil, err
		}

		iTargets, err := s.imgTargetsForDependencyIDs(svc.DependencyIDs)
		if err != nil {
			return nil, errors.Wrapf(err, "getting image build info for %s", svc.Name)
		}
		m = m.WithImageTargets(iTargets)

		result = append(result, m)

		// TODO(maia): might get config files from dc.yml that are overridden by imageTarget :-/
		// e.g. dc.yml specifies one Dockerfile but the imageTarget specifies another
		s.configFiles = sliceutils.DedupedAndSorted(append(s.configFiles, configFiles...))
	}
	if dc.configPath != "" {
		s.configFiles = sliceutils.DedupedAndSorted(append(s.configFiles, dc.configPath))
	}
	return result, nil
}

func badTypeErr(b *starlark.Builtin, ex interface{}, v starlark.Value) error {
	return fmt.Errorf("%v expects a %T; got %T (%v)", b.Name(), ex, v, v)
}

type claim int

const (
	claimNone claim = iota
	claimPending
	claimFinished
)
