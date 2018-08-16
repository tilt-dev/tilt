package service_test

import (
	"testing"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"

	"github.com/stretchr/testify/assert"
)

func TestAddService(t *testing.T) {
	f := newServiceManagerFixture(t)

	s := model.Service{Name: model.ServiceName("hello")}
	err := f.sm.AddService(s)
	assert.NoError(t, err)

	f.AssertServiceList([]model.Service{s})
}

func TestAddManyServices(t *testing.T) {
	f := newServiceManagerFixture(t)

	s1 := model.Service{Name: model.ServiceName("hello")}
	s2 := model.Service{Name: model.ServiceName("world")}
	s3 := model.Service{Name: model.ServiceName("name")}
	err := f.sm.AddService(s1)
	assert.NoError(t, err)
	err = f.sm.AddService(s2)
	assert.NoError(t, err)
	err = f.sm.AddService(s3)
	assert.NoError(t, err)

	expected := []model.Service{s1, s2, s3}

	f.AssertServiceList(expected)
}

func TestAddDuplicateService(t *testing.T) {
	f := newServiceManagerFixture(t)

	s := model.Service{Name: model.ServiceName("hello")}
	err := f.sm.AddService(s)
	assert.NoError(t, err)
	err = f.sm.AddService(s)
	assert.Error(t, err)

	f.AssertServiceList([]model.Service{s})
}

func TestRemoveService(t *testing.T) {
	f := newServiceManagerFixture(t)

	name := model.ServiceName("hello")
	s := model.Service{Name: name}
	err := f.sm.AddService(s)
	assert.NoError(t, err)
	f.sm.RemoveService(name)

	f.AssertServiceList([]model.Service{})
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
	assert.ElementsMatch(f.t, f.sm.List(), s)
}
