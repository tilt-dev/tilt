package image

import (
	"encoding/json"
	"fmt"
	"os"

	digest "github.com/opencontainers/go-digest"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

const imagesPath = "tilt/images.json"

type diskEntry struct {
	Ref          refKey
	Digest       digest.Digest
	CheckpointID CheckpointID
}

func historyFromFS(dir *dirs.WindmillDir) (map[refKey][]historyEntry, error) {
	file, err := dir.OpenFile(imagesPath, os.O_RDONLY, os.FileMode(0))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("historyFromFS: %v", err)
	}

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

func historyToFS(dir *dirs.WindmillDir, entriesMap map[refKey][]historyEntry) error {
	file, err := dir.OpenFile(imagesPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("historyToFS: %v", err)
	}

	encoder := json.NewEncoder(file)
	for ref, entries := range entriesMap {
		for _, entry := range entries {
			diskEntry := diskEntry{Ref: ref, Digest: entry.Digest, CheckpointID: entry.CheckpointID}
			err := encoder.Encode(diskEntry)
			if err != nil {
				return fmt.Errorf("historyToFS: %v", err)
			}
		}
	}
	return nil
}
