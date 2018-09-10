package service_test

import (
	"testing"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	f := newServiceManagerFixture(t)

	s := model.Service{Name: model.ServiceName("hello")}
	err := f.sm.Add(s)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	f.AssertServiceList([]model.Service{s})
}

func TestAddManyServices(t *testing.T) {
	f := newServiceManagerFixture(t)

	s1 := model.Service{Name: model.ServiceName("hello")}
	s2 := model.Service{Name: model.ServiceName("world")}
	s3 := model.Service{Name: model.ServiceName("name")}
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

	expected := []model.Service{s1, s2, s3}

	f.AssertServiceList(expected)
}

func TestUpdate(t *testing.T) {
	f := newServiceManagerFixture(t)

	s1 := model.Service{Name: model.ServiceName("hello"), DockerfileText: "FROM alpine1"}
	err := f.sm.Add(s1)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	s1.DockerfileText = "FROM alpine2"
	err = f.sm.Update(s1)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	f.AssertServiceList([]model.Service{s1})
}

func TestUpdateNonexistantService(t *testing.T) {
	f := newServiceManagerFixture(t)

	s1 := model.Service{Name: model.ServiceName("hello"), DockerfileText: "FROM alpine1"}
	err := f.sm.Add(s1)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	s2 := model.Service{Name: model.ServiceName("hi"), DockerfileText: "FROM alpine2"}
	err = f.sm.Update(s2)
	assert.Error(t, err)

	f.AssertServiceList([]model.Service{s1})
}

func TestAddDuplicateService(t *testing.T) {
	f := newServiceManagerFixture(t)

	s := model.Service{Name: model.ServiceName("hello")}
	err := f.sm.Add(s)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	err = f.sm.Add(s)
	assert.Error(t, err)

	f.AssertServiceList([]model.Service{s})
}

func TestRemoveService(t *testing.T) {
	f := newServiceManagerFixture(t)

	name := model.ServiceName("hello")
	s := model.Service{Name: name}
	err := f.sm.Add(s)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	f.sm.Remove(name)

	f.AssertServiceList([]model.Service{})
}

func TestGetService(t *testing.T) {
	f := newServiceManagerFixture(t)

	name := model.ServiceName("hello")
	expected := model.Service{Name: name}
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

func (f *serviceManagerFixture) AssertServiceList(s []model.Service) {
	f.t.Helper()
	assert.ElementsMatch(f.t, f.sm.List(), s)
}
