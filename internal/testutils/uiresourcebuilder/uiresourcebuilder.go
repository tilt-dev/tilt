package uiresourcebuilder

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type UIResourceBuilder struct {
	name          string
	disabledCount int
	disableSource *v1alpha1.DisableSource
	labels        map[string]string
}

func New(name string) *UIResourceBuilder {
	return &UIResourceBuilder{
		name:   name,
		labels: make(map[string]string),
	}
}

func (u *UIResourceBuilder) WithDisabledCount(i int) *UIResourceBuilder {
	u.disabledCount = i
	return u
}

func (u *UIResourceBuilder) WithDisableSource(s v1alpha1.DisableSource) *UIResourceBuilder {
	u.disableSource = &s
	return u
}

func (u *UIResourceBuilder) WithLabel(l string) *UIResourceBuilder {
	u.labels[l] = l
	return u
}

func (u *UIResourceBuilder) Build() *v1alpha1.UIResource {
	result := &v1alpha1.UIResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:   u.name,
			Labels: u.labels,
		},
		Status: v1alpha1.UIResourceStatus{
			DisableStatus: v1alpha1.DisableResourceStatus{
				DisabledCount: int32(u.disabledCount),
			},
		},
	}
	if u.disableSource != nil {
		result.Status.DisableStatus.Sources = append(result.Status.DisableStatus.Sources, *u.disableSource)
	}

	return result
}
