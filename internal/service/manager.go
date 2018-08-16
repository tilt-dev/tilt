package service

import (
	"fmt"
	"sync"

	"github.com/windmilleng/tilt/internal/model"
)

type Manager struct {
	mu       *sync.Mutex
	services map[model.ServiceName]model.Service
}

func NewManager() *Manager {
	m := make(map[model.ServiceName]model.Service)
	return &Manager{mu: &sync.Mutex{}, services: m}
}

func (m *Manager) AddService(s model.Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.serviceExists(s.Name) {
		return fmt.Errorf("Service %s already exists", s.Name)
	}

	m.services[s.Name] = s

	return nil
}

func (m *Manager) serviceExists(n model.ServiceName) bool {
	_, servicePresent := m.services[n]
	return servicePresent
}

func (m *Manager) List() []model.Service {
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

func (m *Manager) RemoveService(n model.ServiceName) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.services, n)
}
