package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Instance struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	DataDir string `json:"data_dir"`
	PID     int    `json:"pid"`
}

type Manager struct {
	mu        sync.RWMutex
	dir       string
	instances map[string]*Instance
}

func NewManager(name string) (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".rexadb", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	m := &Manager{
		dir:       dir,
		instances: make(map[string]*Instance),
	}
	if err := m.load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return m, nil
}

func (m *Manager) path() string {
	return filepath.Join(m.dir, "instances.json")
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.path())
	if err != nil {
		return err
	}
	var instances []*Instance
	if err := json.Unmarshal(data, &instances); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, inst := range instances {
		m.instances[inst.Name] = inst
	}
	return nil
}

func (m *Manager) save(instances []*Instance) error {
	data, err := json.MarshalIndent(instances, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path(), data, 0644)
}

func (m *Manager) Add(inst *Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[inst.Name]; exists {
		return fmt.Errorf("instance %q already exists", inst.Name)
	}
	m.instances[inst.Name] = inst

	instances := make([]*Instance, 0, len(m.instances))
	for _, i := range m.instances {
		instances = append(instances, i)
	}
	return m.save(instances)
}

func (m *Manager) Get(name string) (*Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[name]
	return inst, ok
}

func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[name]; !exists {
		return fmt.Errorf("instance %q not found", name)
	}
	delete(m.instances, name)

	instances := make([]*Instance, 0, len(m.instances))
	for _, i := range m.instances {
		instances = append(instances, i)
	}
	return m.save(instances)
}

func (m *Manager) List() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	instances := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		instances = append(instances, inst)
	}
	return instances
}

func (m *Manager) Update(name string, fn func(*Instance)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, exists := m.instances[name]
	if !exists {
		return fmt.Errorf("instance %q not found", name)
	}
	fn(inst)

	instances := make([]*Instance, 0, len(m.instances))
	for _, i := range m.instances {
		instances = append(instances, i)
	}
	return m.save(instances)
}
