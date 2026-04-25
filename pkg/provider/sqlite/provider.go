package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rexadb/rexadb/pkg/provider"
)

type SqliteProvider struct {
	manager *Manager
	mu      sync.Mutex
}

func init() {
	provider.Register("sqlite", NewSqliteProvider)
}

func NewSqliteProvider() (provider.Provider, error) {
	mgr, err := newManager("sqlite")
	if err != nil {
		return nil, err
	}

	return &SqliteProvider{
		manager: mgr,
	}, nil
}

type Manager struct {
	mu        sync.RWMutex
	dir       string
	instances map[string]*provider.InstanceInfo
}

func newManager(name string) (*Manager, error) {
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
		instances: make(map[string]*provider.InstanceInfo),
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
	var instances []*provider.InstanceInfo
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

func (m *Manager) save(instances []*provider.InstanceInfo) error {
	data, err := json.MarshalIndent(instances, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path(), data, 0644)
}

func (m *Manager) Add(inst *provider.InstanceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[inst.Name]; exists {
		return fmt.Errorf("instance %q already exists", inst.Name)
	}
	m.instances[inst.Name] = inst

	instances := make([]*provider.InstanceInfo, 0, len(m.instances))
	for _, i := range m.instances {
		instances = append(instances, i)
	}
	return m.save(instances)
}

func (m *Manager) Get(name string) (*provider.InstanceInfo, bool) {
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

	instances := make([]*provider.InstanceInfo, 0, len(m.instances))
	for _, i := range m.instances {
		instances = append(instances, i)
	}
	return m.save(instances)
}

func (m *Manager) List() []*provider.InstanceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	instances := make([]*provider.InstanceInfo, 0, len(m.instances))
	for _, inst := range m.instances {
		instances = append(instances, inst)
	}
	return instances
}

func (m *Manager) Update(name string, fn func(*provider.InstanceInfo)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, exists := m.instances[name]
	if !exists {
		return fmt.Errorf("instance %q not found", name)
	}
	fn(inst)

	instances := make([]*provider.InstanceInfo, 0, len(m.instances))
	for _, i := range m.instances {
		instances = append(instances, i)
	}
	return m.save(instances)
}

func (p *SqliteProvider) Name() string { return "sqlite" }

func (p *SqliteProvider) Start(ctx context.Context, cfg provider.Config, instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cfg.DataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		cfg.DataDir = filepath.Join(home, ".rexadb", "sqlite", instName)
	}

	dbFile := cfg.DataDir
	if cfg.DBName != "" {
		if filepath.Ext(cfg.DBName) != ".db" && filepath.Ext(cfg.DBName) != ".sqlite" {
			dbFile = filepath.Join(cfg.DataDir, cfg.DBName+".db")
		} else {
			dbFile = filepath.Join(cfg.DataDir, cfg.DBName)
		}
	} else {
		dbFile = filepath.Join(cfg.DataDir, instName+".db")
	}

	if err := os.MkdirAll(filepath.Dir(dbFile), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		file, err := os.Create(dbFile)
		if err != nil {
			return fmt.Errorf("failed to create database file: %w", err)
		}
		file.Close()
	}

	inst := &provider.InstanceInfo{
		Name:    instName,
		Type:    p.Name(),
		Host:    "local",
		Port:    0,
		DataDir: dbFile,
	}

	if err := p.manager.Add(inst); err != nil {
		return err
	}

	return nil
}

func (p *SqliteProvider) Stop(instName string) error {
	return nil
}

func (p *SqliteProvider) Delete(instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, exists := p.manager.Get(instName)
	if !exists {
		return fmt.Errorf("instance %q not found", instName)
	}

	return p.manager.Remove(instName)
}

func (p *SqliteProvider) Status(instName string) (bool, error) {
	_, exists := p.manager.Get(instName)
	return exists, nil
}

func (p *SqliteProvider) List() []*provider.InstanceInfo {
	return p.manager.List()
}

func (p *SqliteProvider) IsRunning(dataDir string) bool {
	_, err := os.Stat(dataDir)
	return err == nil
}

func (p *SqliteProvider) GetInstance(instName string) (*provider.InstanceInfo, bool) {
	return p.manager.Get(instName)
}

func (p *SqliteProvider) ConnectionString(cfg provider.Config) string {
	dbFile := cfg.DataDir
	if cfg.DBName != "" {
		dbFile = filepath.Join(cfg.DataDir, cfg.DBName)
	}
	return "sqlite:" + dbFile
}
