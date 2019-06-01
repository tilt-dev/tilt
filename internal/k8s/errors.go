package k8s

import (
	"net/http"

	"k8s.io/apimachinery/pkg/api/errors"
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
