package apis

import (
	"encoding/hex"
	"hash/fnv"
	"net/url"
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/validation"
)

const MaxNameLength = validation.DNS1123SubdomainMaxLength

var invalidLabelCharacters = regexp.MustCompile("[^-A-Za-z0-9_.]")

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

	if len(path.IsValidPathSegmentName(name)) != 0 {
		for _, invalidName := range path.NameMayNotBe {
			if name == invalidName {
				// the only strictly invalid names are `.` and `..` so this is sufficient
				return strings.ReplaceAll(name, ".", "_")
			}
		}

		sanitized = url.QueryEscape(sanitized)
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
