package build

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type ImageReaper struct {
	docker DockerClient
}

func FilterByLabel(label Label) filters.KeyValuePair {
	return filters.Arg("label", string(label))
}

func FilterByLabelValue(label Label, val LabelValue) filters.KeyValuePair {
	return filters.Arg("label", fmt.Sprintf("%s=%s", label, val))
}

func NewImageReaper(docker DockerClient) ImageReaper {
	return ImageReaper{
		docker: docker,
	}
}

// Delete all Tilt builds
//
// For safety reasons, we only delete images with the tilt.buildMode label,
// but we let the caller set additional filters.
func (r ImageReaper) RemoveTiltImages(ctx context.Context, createdBefore time.Time, extraFilters ...filters.KeyValuePair) error {
	defaultFilter := FilterByLabel(BuildMode)
	filterList := append([]filters.KeyValuePair{defaultFilter}, extraFilters...)
	listOptions := types.ImageListOptions{
		Filters: filters.NewArgs(filterList...),
	}

	summaries, err := r.docker.ImageList(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("RemoveTiltImages: %v", err)
	}

	g, ctx := errgroup.WithContext(ctx)
	rmOptions := types.ImageRemoveOptions{
		PruneChildren: true,
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
		return fmt.Errorf("RemoveTiltImages: %v", err)
	}
	return err
}
