package service_test

import (
	"testing"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	f := newServiceManagerFixture(t)

	s := model.Manifest{Name: model.ManifestName("hello")}
	err := f.sm.Add(s)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	f.AssertServiceList([]model.Manifest{s})
}

func TestAddManyServices(t *testing.T) {
	f := newServiceManagerFixture(t)

	s1 := model.Manifest{Name: model.ManifestName("hello")}
	s2 := model.Manifest{Name: model.ManifestName("world")}
	s3 := model.Manifest{Name: model.ManifestName("name")}
	err := f.sm.Add(s1)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	err = f.sm.Add(s2)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	err = f.sm.Add(s3)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	expected := []model.Manifest{s1, s2, s3}

	f.AssertServiceList(expected)
}

func TestUpdate(t *testing.T) {
	f := newServiceManagerFixture(t)

	s1 := model.Manifest{Name: model.ManifestName("hello"), DockerfileText: "FROM alpine1"}
	err := f.sm.Add(s1)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	s1.DockerfileText = "FROM alpine2"
	err = f.sm.Update(s1)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	f.AssertServiceList([]model.Manifest{s1})
}

func TestUpdateNonexistantService(t *testing.T) {
	f := newServiceManagerFixture(t)

	s1 := model.Manifest{Name: model.ManifestName("hello"), DockerfileText: "FROM alpine1"}
	err := f.sm.Add(s1)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	s2 := model.Manifest{Name: model.ManifestName("hi"), DockerfileText: "FROM alpine2"}
	err = f.sm.Update(s2)
	assert.Error(t, err)

	f.AssertServiceList([]model.Manifest{s1})
}

func TestAddDuplicateService(t *testing.T) {
	f := newServiceManagerFixture(t)

	s := model.Manifest{Name: model.ManifestName("hello")}
	err := f.sm.Add(s)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	err = f.sm.Add(s)
	assert.Error(t, err)

	f.AssertServiceList([]model.Manifest{s})
}

func TestRemoveService(t *testing.T) {
	f := newServiceManagerFixture(t)

	name := model.ManifestName("hello")
	s := model.Manifest{Name: name}
	err := f.sm.Add(s)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	f.sm.Remove(name)

	f.AssertServiceList([]model.Manifest{})
}

func TestGetService(t *testing.T) {
	f := newServiceManagerFixture(t)

	name := model.ManifestName("hello")
	expected := model.Manifest{Name: name}
	err := f.sm.Add(expected)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	actual, err := f.sm.Get(expected.Name)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	if !assert.EqualValues(t, actual, expected) {
		t.Errorf("Expected %+v to equal %+v", actual, expected)
	}
}

type serviceManagerFixture struct {
	t  *testing.T
	sm service.Manager
}

func newServiceManagerFixture(t *testing.T) *serviceManagerFixture {
	sm := service.NewMemoryManager()

	return &serviceManagerFixture{
		t:  t,
		sm: sm,
	}
}

func (f *serviceManagerFixture) AssertServiceList(s []model.Manifest) {
	assert.ElementsMatch(f.t, f.sm.List(), s)
}
