package containerizedengine

import (
	"context"
	"sort"

	registryclient "github.com/docker/cli/cli/registry/client"
	"github.com/docker/distribution/reference"
	ver "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetEngineVersions reports the versions of the engine that are available
func (c baseClient) GetEngineVersions(ctx context.Context, registryClient registryclient.RegistryClient, currentVersion, imageName string) (AvailableVersions, error) {
	imageRef, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return AvailableVersions{}, err
	}

	tags, err := registryClient.GetTags(ctx, imageRef)
	if err != nil {
		return AvailableVersions{}, err
	}

	return parseTags(tags, currentVersion)
}

func parseTags(tags []string, currentVersion string) (AvailableVersions, error) {
	var ret AvailableVersions
	currentVer, err := ver.NewVersion(currentVersion)
	if err != nil {
		return ret, errors.Wrapf(err, "failed to parse existing version %s", currentVersion)
	}
	downgrades := []DockerVersion{}
	patches := []DockerVersion{}
	upgrades := []DockerVersion{}
	currentSegments := currentVer.Segments()
	for _, tag := range tags {
		tmp, err := ver.NewVersion(tag)
		if err != nil {
			logrus.Debugf("Unable to parse %s: %s", tag, err)
			continue
		}
		testVersion := DockerVersion{Version: *tmp, Tag: tag}
		if testVersion.LessThan(currentVer) {
			downgrades = append(downgrades, testVersion)
			continue
		}
		testSegments := testVersion.Segments()
		// lib always provides min 3 segments
		if testSegments[0] == currentSegments[0] &&
			testSegments[1] == currentSegments[1] {
			patches = append(patches, testVersion)
		} else {
			upgrades = append(upgrades, testVersion)
		}
	}
	sort.Slice(downgrades, func(i, j int) bool {
		return downgrades[i].Version.LessThan(&downgrades[j].Version)
	})
	sort.Slice(patches, func(i, j int) bool {
		return patches[i].Version.LessThan(&patches[j].Version)
	})
	sort.Slice(upgrades, func(i, j int) bool {
		return upgrades[i].Version.LessThan(&upgrades[j].Version)
	})
	ret.Downgrades = downgrades
	ret.Patches = patches
	ret.Upgrades = upgrades
	return ret, nil
}
