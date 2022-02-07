package apiset

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestAddSetForTypeNoExistingEntries(t *testing.T) {
	a := &v1alpha1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	b := &v1alpha1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "b"}}
	c := &v1alpha1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "c"}}
	set := make(ObjectSet)
	set.Add(a)

	newObjs := make(TypedObjectSet)
	newObjs["b"] = b
	newObjs["c"] = c

	set.AddSetForType(&v1alpha1.ConfigMap{}, newObjs)

	var observed []*v1alpha1.ConfigMap
	for _, v := range set.GetSetForType(&v1alpha1.ConfigMap{}) {
		observed = append(observed, v.(*v1alpha1.ConfigMap))
	}

	require.ElementsMatch(t, []*v1alpha1.ConfigMap{a, b, c}, observed)

}

func TestAddSetForTypeKeepsOldEntries(t *testing.T) {
	a := &v1alpha1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	b := &v1alpha1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "b"}}
	c := &v1alpha1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "c"}}
	set := make(ObjectSet)
	set.Add(a)

	newObjs := make(TypedObjectSet)
	newObjs["b"] = b
	newObjs["c"] = c

	set.AddSetForType(&v1alpha1.ConfigMap{}, newObjs)

	var observed []*v1alpha1.ConfigMap
	for _, v := range set.GetSetForType(&v1alpha1.ConfigMap{}) {
		observed = append(observed, v.(*v1alpha1.ConfigMap))
	}

	require.ElementsMatch(t, []*v1alpha1.ConfigMap{a, b, c}, observed)
}
