package apiclient

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	clienttest "k8s.io/client-go/testing"

	"github.com/tilt-dev/tilt/pkg/clientset/tiltapi/typed/core/v1alpha1/fake"
)

type ActionMatchFunc func(action clienttest.Action) bool
type UpdateMatchFunc func(obj runtime.Object) bool

type FakeClientFixture struct {
	t testing.TB

	Fake *clienttest.Fake
}

func NewFakeClientFixtureForFake(t testing.TB, fake *clienttest.Fake) *FakeClientFixture {
	t.Helper()

	t.Cleanup(fake.ClearActions)

	f := &FakeClientFixture{
		t:    t,
		Fake: fake,
	}

	return f
}

func NewFakeClientFixture(t testing.TB) *FakeClientFixture {
	t.Helper()
	return NewFakeClientFixtureForFake(t, &clienttest.Fake{})
}

func (f *FakeClientFixture) CoreV1alpha1() *fake.FakeCoreV1alpha1 {
	return &fake.FakeCoreV1alpha1{Fake: f.Fake}
}

func (f *FakeClientFixture) AssertCreated(name string, resourceType string) runtime.Object {
	f.t.Helper()
	var createdObj runtime.Object
	f.AssertActionsContains(func(action clienttest.Action) bool {
		f.t.Helper()
		if !action.Matches("create", resourceType) {
			return false
		}
		createAction, ok := action.(clienttest.CreateAction)
		if !ok {
			f.t.Fatalf("Received create action of type %T", action)
		}
		objMeta, err := meta.Accessor(createAction.GetObject())
		require.NoError(f.t, err)
		if objMeta.GetName() == name {
			createdObj = createAction.GetObject()
			return true
		}
		return false
	})
	return createdObj
}

func (f *FakeClientFixture) AssertDeleted(name string, resourceType string) {
	f.t.Helper()
	f.AssertActionsContains(func(action clienttest.Action) bool {
		f.t.Helper()
		if !action.Matches("delete", resourceType) {
			return false
		}
		deleteAction, ok := action.(clienttest.DeleteAction)
		if !ok {
			f.t.Fatalf("Received delete action of type %T", action)
		}
		return deleteAction.GetName() == name
	})
}

// AssertUpdated asserts that a clienttest.UpdateAction that matches the conditions has been received.
//
// NOTE: When updating subresources (e.g. with UpdateStatus), the entity will ONLY have the subresource populated,
//		 so the name of the subresource should be passed instead and the UpdateMatchFunc should only examine fields on
//		 subresource as well.
func (f *FakeClientFixture) AssertUpdated(nameOrSubresource string, resourceType string, matchFunc UpdateMatchFunc) runtime.Object {
	f.t.Helper()
	var obj runtime.Object
	f.AssertActionsContains(func(action clienttest.Action) bool {
		if !action.Matches("update", resourceType) {
			return false
		}
		updateAction, ok := action.(clienttest.UpdateAction)
		if !ok {
			f.t.Fatalf("Received update action of type %T", action)
		}
		objMeta, err := meta.Accessor(updateAction.GetObject())
		require.NoError(f.t, err)
		if objMeta.GetName() != nameOrSubresource && updateAction.GetSubresource() != nameOrSubresource {
			return false
		}
		if !matchFunc(updateAction.GetObject()) {
			return false
		}
		obj = updateAction.GetObject()
		return true
	})
	return obj
}

func (f *FakeClientFixture) AssertActionsContains(matchFunc ActionMatchFunc) clienttest.Action {
	f.t.Helper()

	timeout := time.After(1 * time.Second)
	for {
		select {
		case <-time.After(10 * time.Millisecond):
			for _, action := range f.Fake.Actions() {
				if matchFunc(action) {
					return action
				}
			}
		case <-timeout:
			f.t.Fatalf("No action matched within timeout, seen:\n%s", actionsForLog(f.Fake.Actions()))
		}
	}
}

type actionsForLog []clienttest.Action

func (a actionsForLog) String() string {
	var out []string
	for i, action := range a {
		var name string
		switch x := action.(type) {
		case clienttest.CreateAction:
			if metaObj, _ := meta.Accessor(x.GetObject()); metaObj != nil {
				name = metaObj.GetName()
			}
		case clienttest.GetAction:
			name = x.GetName()
		case clienttest.UpdateAction:
			if metaObj, _ := meta.Accessor(x.GetObject()); metaObj != nil {
				name = metaObj.GetName()
			}
		case clienttest.PatchAction:
			name = x.GetName()
		case clienttest.DeleteAction:
			name = x.GetName()
		}
		m := fmt.Sprintf("\t%d) %s %s:%s", i+1, strings.ToUpper(action.GetVerb()), action.GetResource().Resource, action.GetSubresource())
		if name != "" {
			// when performing subresource updates, the name is unfortunately not present
			m += ":" + name
		}
		out = append(out, m)
	}
	return strings.Join(out, "\n")
}
