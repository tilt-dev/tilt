package yaml

import (
	"fmt"
	"strings"
)

const separator = "---"

func ConcatYaml(yaml ...string) string {
	if len(yaml) == 0 {
		return ""
	} else if len(yaml) == 1 {
		return yaml[0]
	}

	result := yaml[0]
	for _, y := range yaml[1:] {
		result = concatYaml(result, y)
	}
	return result
}

func concatYaml(y1, y2 string) string {
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
