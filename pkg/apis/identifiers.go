package apis

import (
	"encoding/hex"
	"hash/fnv"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"

	"k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/validation"
)

const MaxNameLength = validation.DNS1123SubdomainMaxLength

var invalidPathCharacters = regexp.MustCompile(`[` + strings.Join(path.NameMayNotContain, "") + `]`)

func Key(o resource.Object) types.NamespacedName {
	objMeta := o.GetObjectMeta()
	return types.NamespacedName{Name: objMeta.Name, Namespace: objMeta.Namespace}
}

// SanitizeName ensures a value is suitable for usage as an apiserver identifier.
func SanitizeName(name string) string {
	sanitized := name
	if len(path.IsValidPathSegmentName(name)) != 0 {
		for _, invalidName := range path.NameMayNotBe {
			if name == invalidName {
				// the only strictly invalid names are `.` and `..` so this is sufficient
				return strings.ReplaceAll(name, ".", "_")
			}
		}
		sanitized = invalidPathCharacters.ReplaceAllString(sanitized, "_")
	}
	if len(sanitized) > MaxNameLength {
		var sb strings.Builder
		sb.Grow(MaxNameLength)
		sb.WriteString(sanitized[:MaxNameLength-9])
		sb.WriteRune('-')
		sb.WriteString(hashValue(name))
		sanitized = sb.String()
	}
	return sanitized
}

func hashValue(v string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(v))
	return hex.EncodeToString(h.Sum(nil))
}
