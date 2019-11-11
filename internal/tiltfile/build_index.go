package tiltfile

import (
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/schollz/closestmatch"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/pkg/model"
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
	selector := img.configurationRef
	name := selector.RefName()
	_, hasExisting := idx.imagesBySelector[selector.String()]
	if hasExisting {
		return fmt.Errorf("Image for ref %q has already been defined", container.FamiliarString(selector))
	}

	idx.imagesBySelector[selector.String()] = img

	_, hasExistingName := idx.imagesByName[name]
	if hasExistingName {
		// If the two selectors have the same name but different refs, they must
		// have different tags. Make all the selectors "exact", so that they
		// only match the exact tag.
		img.configurationRef = img.configurationRef.WithExactMatch()

		for _, image := range idx.images {
			if image.configurationRef.RefName() == name {
				image.configurationRef = image.configurationRef.WithExactMatch()
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
	name := reference.FamiliarName(ref)
	if !idx.consumedImageNameMap[name] {
		idx.consumedImageNameMap[name] = true
		idx.consumedImageNames = append(idx.consumedImageNames, name)
	}

	for _, image := range idx.images {
		if image.configurationRef.Matches(ref) {
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
			for i, match := range cm.ClosestN(image.configurationRef.RefFamiliarName(), 3) {
				// If there are no matches, the closestmatch library sometimes returns
				// an empty string
				if match == "" {
					break
				}
				if i == 0 {
					matchLines = append(matchLines, "Did you mean…\n")
				}
				matchLines = append(matchLines, fmt.Sprintf("    - %s\n", match))
			}

			return fmt.Errorf("Image not used in any deploy config:\n    ✕ %v\n%sSkipping this image build",
				container.FamiliarString(image.configurationRef), strings.Join(matchLines, ""))
		}
	}
	return nil
}
