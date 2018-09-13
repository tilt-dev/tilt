package service

import (
	"fmt"
	"sync"

	"github.com/windmilleng/tilt/internal/model"
)

type Manager interface {
	Add(s model.Manifest) error
	Update(s model.Manifest) error
	Remove(s model.ManifestName)
	List() []model.Manifest
	Get(n model.ManifestName) (model.Manifest, error)
}

type memoryManager struct {
	mu       *sync.Mutex
	services map[model.ManifestName]model.Manifest
}

func ProvideMemoryManager() Manager {
	return NewMemoryManager()
}

func NewMemoryManager() *memoryManager {
	m := make(map[model.ManifestName]model.Manifest)
	return &memoryManager{mu: &sync.Mutex{}, services: m}
}

func (m *memoryManager) Add(s model.Manifest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.serviceExists(s.Name) {
		return fmt.Errorf("Service %s already exists", s.Name)
	}

	m.services[s.Name] = s

	return nil
}

func (m *memoryManager) Update(s model.Manifest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.serviceExists(s.Name) {
		return fmt.Errorf("Service %s doesn't exist", s.Name)
	}

	m.services[s.Name] = s

	return nil
}

func (m *memoryManager) serviceExists(n model.ManifestName) bool {
	_, servicePresent := m.services[n]
	return servicePresent
}

func (m *memoryManager) List() []model.Manifest {
	m.mu.Lock()
	defer m.mu.Unlock()

	v := make([]model.Manifest, len(m.services))

	i := 0
	for _, s := range m.services {
		v[i] = s
		i++
	}

	return v
}

func (m *memoryManager) Remove(n model.ManifestName) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.services, n)
}

func (m *memoryManager) Get(n model.ManifestName) (model.Manifest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.serviceExists(n) {
		return m.services[n], nil
	}

	return model.Manifest{}, fmt.Errorf("Unable to find service %s", n)
}
