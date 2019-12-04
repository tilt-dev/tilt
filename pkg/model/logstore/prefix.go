package logstore

import (
	"fmt"
	"strings"

	"github.com/windmilleng/tilt/pkg/model"
)

func SourcePrefix(n model.ManifestName) string {
	max := 12
	spaces := ""
	if len(n) > max {
		n = n[:max-1] + "…"
	} else {
		spaces = strings.Repeat(" ", max-len(n))
	}
	return fmt.Sprintf("%s%s┊ ", n, spaces)
}
