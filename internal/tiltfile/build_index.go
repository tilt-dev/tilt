package tiltfile

import (
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/schollz/closestmatch"
	"github.com/windmilleng/tilt/internal/model"
)

// An index of all the images that we know how to build.
type buildIndex struct {
	// keep a slice so that the order of iteration is deterministic
	images []*dockerImage

	taggedImages   map[string]*dockerImage
	nameOnlyImages map[string]*dockerImage
	byTargetID     map[model.TargetID]*dockerImage

	consumedImageNames   []string
	consumedImageNameMap map[string]bool
}

func newBuildIndex() *buildIndex {
	return &buildIndex{
		taggedImages:         make(map[string]*dockerImage),
		nameOnlyImages:       make(map[string]*dockerImage),
		byTargetID:           make(map[model.TargetID]*dockerImage),
		consumedImageNameMap: make(map[string]bool),
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

	idx.byTargetID[img.ID()] = img

	idx.images = append(idx.images, img)
	return nil
}

func (idx *buildIndex) findBuilderByID(id model.TargetID) *dockerImage {
	return idx.byTargetID[id]
}

// Many things can consume image builds:
// - k8s yaml
// - docker-compose yaml
// - the from command in other images
// Check to see if we have a build target for that image,
// and mark that build target as consumed by an larger target.
func (idx *buildIndex) findBuilderForConsumedImage(ref reference.Named) *dockerImage {
	if !idx.consumedImageNameMap[ref.Name()] {
		idx.consumedImageNameMap[ref.Name()] = true
		idx.consumedImageNames = append(idx.consumedImageNames, ref.Name())
	}

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
			bagSizes := []int{2, 3, 4}
			cm := closestmatch.New(idx.consumedImageNames, bagSizes)
			matchLines := []string{}
			for i, match := range cm.ClosestN(image.ref.Name(), 3) {
				if i == 0 {
					matchLines = append(matchLines, "Did you mean:")
				}
				matchLines = append(matchLines, fmt.Sprintf(" - %s", match))
			}

			return fmt.Errorf("image %v is not used in any resource. %s",
				image.ref.String(), strings.Join(matchLines, "\n"))
		}
	}
	return nil
}
