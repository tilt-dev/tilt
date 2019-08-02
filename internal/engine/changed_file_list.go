package engine

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/ospath"
)

// When we kick off a build because some files changed, only print the first `maxChangedFilesToPrint`
const maxChangedFilesToPrint = 5

func formatFileChangeList(changedFiles []string) string {
	var changedPathsToPrint []string
	if len(changedFiles) > maxChangedFilesToPrint {
		changedPathsToPrint = append(changedPathsToPrint, changedFiles[:maxChangedFilesToPrint]...)
		changedPathsToPrint = append(changedPathsToPrint, "...")
	} else {
		changedPathsToPrint = changedFiles
	}

	return fmt.Sprintf("%v", ospath.TryAsCwdChildren(changedFiles))
}
