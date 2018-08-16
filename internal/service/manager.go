package service

import (
	"fmt"
	"sync"

	"github.com/windmilleng/tilt/internal/model"
)

type Manager interface {
	AddService(s model.Service) error
	RemoveService(s model.ServiceName)
	List() []model.Service
}

type memoryManager struct {
	mu       *sync.Mutex
	services map[model.ServiceName]model.Service
}

func NewMemoryManager() *memoryManager {
	m := make(map[model.ServiceName]model.Service)
	return &memoryManager{mu: &sync.Mutex{}, services: m}
}

func (m *memoryManager) AddService(s model.Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.serviceExists(s.Name) {
		return fmt.Errorf("Service %s already exists", s.Name)
	}

	m.services[s.Name] = s

	return nil
}

func (m *memoryManager) serviceExists(n model.ServiceName) bool {
	_, servicePresent := m.services[n]
	return servicePresent
}

func (m *memoryManager) List() []model.Service {
	m.mu.Lock()
	defer m.mu.Unlock()

	v := make([]model.Service, len(m.services))
	fmt.Printf("%+v\n", len(v))

	i := 0
	for _, s := range m.services {
		v[i] = s
		i++
	}

	return v
}

func (m *memoryManager) RemoveService(n model.ServiceName) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.services, n)
}
