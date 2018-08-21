package image

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	digest "github.com/opencontainers/go-digest"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

const imagesPath = "tilt/images.json"

type diskEntry struct {
	Ref          refKey
	Digest       digest.Digest
	CheckpointID CheckpointID
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

		result[entry.Ref] = append(result[entry.Ref],
			historyEntry{CheckpointID: entry.CheckpointID, Digest: entry.Digest})
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
	diskEntry := diskEntry{Ref: ref, Digest: entry.Digest, CheckpointID: entry.CheckpointID}
	err = encoder.Encode(diskEntry)
	if err != nil {
		return fmt.Errorf("addHistoryToFS: %v", err)
	}
	return nil
}
