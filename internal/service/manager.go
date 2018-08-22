package service

import (
	"fmt"
	"sync"

	"github.com/windmilleng/tilt/internal/model"
)

type Manager interface {
	Add(s model.Service) error
	Update(s model.Service) error
	Remove(s model.ServiceName)
	List() []model.Service
	Get(n model.ServiceName) (model.Service, error)
}

type memoryManager struct {
	mu       *sync.Mutex
	services map[model.ServiceName]model.Service
}

func ProvideMemoryManager() Manager {
	return NewMemoryManager()
}

func NewMemoryManager() *memoryManager {
	m := make(map[model.ServiceName]model.Service)
	return &memoryManager{mu: &sync.Mutex{}, services: m}
}

func (m *memoryManager) Add(s model.Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.serviceExists(s.Name) {
		return fmt.Errorf("Service %s already exists", s.Name)
	}

	m.services[s.Name] = s

	return nil
}

func (m *memoryManager) Update(s model.Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.serviceExists(s.Name) {
		return fmt.Errorf("Service %s doesn't exist", s.Name)
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

	i := 0
	for _, s := range m.services {
		v[i] = s
		i++
	}

	return v
}

func (m *memoryManager) Remove(n model.ServiceName) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.services, n)
}

func (m *memoryManager) Get(n model.ServiceName) (model.Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.serviceExists(n) {
		return m.services[n], nil
	}

	return model.Service{}, fmt.Errorf("Unable to find service %s", n)
}
