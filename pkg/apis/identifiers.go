package apis

import (
	"encoding/hex"
	"hash/fnv"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/validate/content"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/util/validation"
)

const MaxNameLength = validation.DNS1123SubdomainMaxLength

// Strings that cannot be used as names specified as path segments (like the
// REST API or etcd store).
var pathSegmentNameMayNotBe = []string{".", ".."}

// Substrings that cannot be used in names specified as path segments (like the
// REST API or etcd store).
var pathSegmentNameMayNotContain = []string{"/", "%"}

var invalidLabelCharacters = regexp.MustCompile("[^-A-Za-z0-9_.]")

var invalidPathCharacters = regexp.MustCompile(`[` + strings.Join(pathSegmentNameMayNotContain, "") + `]`)

type KeyableObject interface {
	GetName() string
	GetNamespace() string
}

func Key(o KeyableObject) types.NamespacedName {
	return types.NamespacedName{Name: o.GetName(), Namespace: o.GetNamespace()}
}

func KeyFromMeta(objMeta metav1.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{Name: objMeta.Name, Namespace: objMeta.Namespace}
}

// SanitizeLabel ensures a value is suitable as both a label key and value.
func SanitizeLabel(name string) string {
	sanitized := invalidLabelCharacters.ReplaceAllString(name, "_")
	max := validation.LabelValueMaxLength
	if len(sanitized) > max {
		var sb strings.Builder
		sb.Grow(max)
		sb.WriteString(sanitized[:max-9])
		sb.WriteRune('-')
		sb.WriteString(hashValue(name))
		sanitized = sb.String()
	}
	return sanitized
}

// SanitizeName ensures a value is suitable for usage as an apiserver identifier.
func SanitizeName(name string) string {
	sanitized := name
	if len(content.IsPathSegmentName(name)) != 0 {
		for _, invalidName := range pathSegmentNameMayNotBe {
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
