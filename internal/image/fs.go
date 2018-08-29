package image

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/docker/distribution/reference"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

const imagesPath = "tilt/images.json"

type diskEntry struct {
	Ref          refKey
	CheckpointID CheckpointID
	HashedInputs model.HashedService
}

func historyFromFS(ctx context.Context, dir *dirs.WindmillDir) (map[refKey][]historyEntry, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-historyFromFS")
	defer span.Finish()
	file, err := dir.OpenFile(imagesPath, os.O_RDONLY, os.FileMode(0))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("historyFromFS: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	result := make(map[refKey][]historyEntry, 0)
	decoder := json.NewDecoder(file)
	for decoder.More() {
		entry := diskEntry{}
		err := decoder.Decode(&entry)
		if err != nil {
			return nil, fmt.Errorf("historyFromFS: %v", err)
		}

		name, err := reference.ParseNamed(string(entry.Ref))
		if err != nil {
			return nil, fmt.Errorf("reference.Parse: %v", err)
		}
		result[entry.Ref] = append(result[entry.Ref],
			historyEntry{
				Named:         name,
				CheckpointID:  entry.CheckpointID,
				HashedService: entry.HashedInputs,
			})
	}
	return result, nil
}

func addHistoryToFS(ctx context.Context, dir *dirs.WindmillDir, ref refKey, entry historyEntry) error {
	file, err := dir.OpenFile(imagesPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("addHistoryToFS: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	encoder := json.NewEncoder(file)
	diskEntry := diskEntry{Ref: ref, CheckpointID: entry.CheckpointID, HashedInputs: entry.HashedService}
	err = encoder.Encode(diskEntry)
	if err != nil {
		return fmt.Errorf("addHistoryToFS: %v", err)
	}
	return nil
}
