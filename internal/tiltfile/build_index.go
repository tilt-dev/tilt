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

	imagesByName     map[string]*dockerImage
	imagesBySelector map[string]*dockerImage
	byTargetID       map[model.TargetID]*dockerImage

	consumedImageNames   []string
	consumedImageNameMap map[string]bool
}

func newBuildIndex() *buildIndex {
	return &buildIndex{
		imagesBySelector:     make(map[string]*dockerImage),
		imagesByName:         make(map[string]*dockerImage),
		byTargetID:           make(map[model.TargetID]*dockerImage),
		consumedImageNameMap: make(map[string]bool),
	}
}

func (idx *buildIndex) addImage(img *dockerImage) error {
	selector := img.ref
	name := selector.RefName()
	_, hasExisting := idx.imagesBySelector[selector.String()]
	if hasExisting {
		return fmt.Errorf("Image for ref %q has already been defined", selector.String())
	}

	idx.imagesBySelector[selector.String()] = img

	_, hasExistingName := idx.imagesByName[name]
	if hasExistingName {
		// If the two selectors have the same name but different refs, they must
		// have different tags. Make all the selectors "exact", so that they
		// only match the exact tag.
		img.ref = img.ref.WithExactMatch()

		for _, image := range idx.images {
			if image.ref.RefName() == name {
				image.ref = image.ref.WithExactMatch()
			}
		}
	}

	idx.imagesByName[name] = img
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

	for _, image := range idx.images {
		if image.ref.Matches(ref) {
			image.matched = true
			return image
		}
	}
	return nil
}

func (idx *buildIndex) assertAllMatched() error {
	for _, image := range idx.images {
		if !image.matched {
			bagSizes := []int{2, 3, 4}
			cm := closestmatch.New(idx.consumedImageNames, bagSizes)
			matchLines := []string{}
			for i, match := range cm.ClosestN(image.ref.RefName(), 3) {
				if i == 0 {
					matchLines = append(matchLines, "Did you mean…")
				}
				matchLines = append(matchLines, fmt.Sprintf("    - %s", match))
			}

			return fmt.Errorf("Image not used in any resource:\n    ✕ %v\n%s",
				image.ref.String(), strings.Join(matchLines, "\n"))
		}
	}
	return nil
}
