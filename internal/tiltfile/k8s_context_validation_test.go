package tiltfile

import (
	"testing"
)

func TestK8sContextOrderValidation_Valid(t *testing.T) {
	content := `
k8s_context('test-context')
k8s_yaml('pod.yaml')
`
	err := validateK8sContextOrder(content, "Tiltfile")
	if err != nil {
		t.Fatalf("Expected no error for valid order, got: %v", err)
	}
}

func TestK8sContextOrderValidation_Invalid(t *testing.T) {
	content := `
k8s_yaml('pod.yaml')
k8s_context('test-context')
`
	err := validateK8sContextOrder(content, "Tiltfile")
	if err == nil {
		t.Fatal("Expected error for invalid order, got nil")
	}

	if !containsString(err.Error(), "k8s_context() with parameters cannot be called after Kubernetes-interacting commands") {
		t.Fatalf("Expected specific error message, got: %v", err)
	}
}

func TestK8sContextOrderValidation_ReadOnlyAllowed(t *testing.T) {
	content := `
k8s_yaml('pod.yaml')
current_ctx = k8s_context()
`
	err := validateK8sContextOrder(content, "Tiltfile")
	if err != nil {
		t.Fatalf("Expected no error for read-only k8s_context(), got: %v", err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && containsString(s[1:], substr)
}
