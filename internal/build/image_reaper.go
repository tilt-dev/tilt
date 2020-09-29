package build

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockerfile"
)

type ImageReaper struct {
	docker docker.Client
}

func FilterByLabel(label dockerfile.Label) filters.KeyValuePair {
	return filters.Arg("label", string(label))
}

func FilterByLabelValue(label dockerfile.Label, val dockerfile.LabelValue) filters.KeyValuePair {
	return filters.Arg("label", fmt.Sprintf("%s=%s", label, val))
}

func FilterByRefName(ref reference.Named) filters.KeyValuePair {
	return filters.Arg("reference", fmt.Sprintf("%s:*", ref.Name()))
}

func NewImageReaper(docker docker.Client) ImageReaper {
	return ImageReaper{
		docker: docker,
	}
}

// Delete all Tilt builds
//
// For safety reasons, we only delete images with the tilt.buildMode label,
// but we let the caller set additional filters.
func (r ImageReaper) RemoveTiltImages(ctx context.Context, createdBefore time.Time, force bool, extraFilters ...filters.KeyValuePair) error {
	defaultFilter := FilterByLabel(BuildMode)
	filterList := append([]filters.KeyValuePair{defaultFilter}, extraFilters...)
	listOptions := types.ImageListOptions{
		Filters: filters.NewArgs(filterList...),
	}

	summaries, err := r.docker.ImageList(ctx, listOptions)
	if err != nil {
		return errors.Wrap(err, "RemoveTiltImages")
	}

	g, ctx := errgroup.WithContext(ctx)
	rmOptions := types.ImageRemoveOptions{
		PruneChildren: true,
		Force:         force,
	}
	for _, summary := range summaries {
		id := summary.ID
		created := time.Unix(summary.Created, 0)
		if !created.Before(createdBefore) {
			continue
		}

		g.Go(func() error {
			_, err := r.docker.ImageRemove(ctx, id, rmOptions)
			if client.IsErrNotFound(err) {
				return nil
			}
			return err
		})
	}

	err = g.Wait()
	if err != nil {
		return errors.Wrap(err, "RemoveTiltImages")
	}
	return err
}
