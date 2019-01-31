package tiltfile

import (
	"fmt"

	"github.com/docker/distribution/reference"
)

// An index of all the images that we know how to build.
type buildIndex struct {
	// keep a slice so that the order of iteration is deterministic
	images []*dockerImage

	taggedImages   map[string]*dockerImage
	nameOnlyImages map[string]*dockerImage
}

func newBuildIndex() *buildIndex {
	return &buildIndex{
		taggedImages:   make(map[string]*dockerImage),
		nameOnlyImages: make(map[string]*dockerImage),
	}
}

func (idx *buildIndex) addImage(img *dockerImage) error {
	ref := img.ref
	refTagged, hasTag := ref.(reference.NamedTagged)
	if hasTag {
		key := fmt.Sprintf("%s:%s", ref.Name(), refTagged.Tag())
		_, exists := idx.taggedImages[key]
		if exists {
			return fmt.Errorf("Image for ref %q has already been defined", key)
		}

		idx.taggedImages[key] = img
	} else {
		key := ref.Name()
		_, exists := idx.nameOnlyImages[key]
		if exists {
			return fmt.Errorf("Image for ref %q has already been defined", key)
		}

		idx.nameOnlyImages[key] = img
	}
	idx.images = append(idx.images, img)
	return nil
}

// Deploy targets (like k8s yaml and docker-compose yaml) have reference to images
// Check to see if we have a build target for that image.
func (idx *buildIndex) matchRefInDeployTarget(ref reference.Named) *dockerImage {
	refTagged, hasTag := ref.(reference.NamedTagged)
	if hasTag {
		key := fmt.Sprintf("%s:%s", ref.Name(), refTagged.Tag())
		img, exists := idx.taggedImages[key]
		if exists {
			img.matched = true
			return img
		}
	}

	key := ref.Name()
	img, exists := idx.nameOnlyImages[key]
	if exists {
		img.matched = true
		return img
	}
	return nil
}

func (idx *buildIndex) assertAllMatched() error {
	for _, image := range idx.images {
		if !image.matched {
			return fmt.Errorf("image %v is not used in any resource", image.ref.String())
		}
	}
	return nil
}
