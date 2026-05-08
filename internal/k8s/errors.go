package k8s

import (
	"net/http"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newForbiddenError() *errors.StatusError {
	return &errors.StatusError{
		ErrStatus: metav1.Status{
			Message: "unknown",
			Reason:  "Forbidden",
			Code:    http.StatusForbidden,
		},
	}
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return apierrors.IsNotFound(err) ||
		// Helm has it's own custom not found error.
		strings.Contains(err.Error(), "object not found") ||
		kubernetesResourceNotFoundRe.MatchString(err.Error())
}

var kubernetesResourceNotFoundRe = regexp.MustCompile(`(?m)\b[a-z0-9.]+ "[^"]+" not found\b`)

func isMissingKindError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no matches for kind")
}
