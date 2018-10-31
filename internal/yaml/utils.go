package yaml

import (
	"fmt"
	"strings"
)

// TODO(maia): store yaml strings as []string so we don't need to parse strings for separators?
const separator = "---"

func ConcatYAML(yaml ...string) string {
	if len(yaml) == 0 {
		return ""
	} else if len(yaml) == 1 {
		return yaml[0]
	}

	result := yaml[0]
	for _, y := range yaml[1:] {
		result = concatYAML(result, y)
	}
	return result
}

func concatYAML(y1, y2 string) string {
	y1 = strings.TrimSpace(y1)
	y2 = strings.TrimSpace(y2)
	if !hasEndingSeparator(y1) && !hasStartingSeparator(y2) {
		return fmt.Sprintf("%s\n%s\n%s", y1, separator, y2)
	} else if hasEndingSeparator(y1) && hasStartingSeparator(y2) {
		y1 = strings.TrimSpace(strings.TrimSuffix(y1, separator))
	}
	return fmt.Sprintf("%s\n%s", y1, y2)
}

func hasStartingSeparator(y string) bool {
	return strings.HasPrefix(y, separator)
}

func hasEndingSeparator(y string) bool {
	return strings.HasSuffix(y, separator)
}
