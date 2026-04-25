package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/rexadb/rexadb/pkg/provider"
)

type MongoProvider struct {
	binPath string
	manager *Manager
	mu      sync.Mutex
}

func init() {
	provider.Register("mongodb", NewMongoProvider)
}

func NewMongoProvider() (provider.Provider, error) {
	mongoBin, err := findMongoBin()
	if err != nil {
		return nil, err
	}

	mgr, err := newManager("mongodb")
	if err != nil {
		return nil, err
	}

	return &MongoProvider{
		binPath: mongoBin,
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

func installMongo() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		if _, err := os.Stat("/etc/debian_version"); err == nil {
			cmd = exec.Command("pkexec", "apt", "install", "-y", "mongodb")
		} else if _, err := os.Stat("/etc/arch-release"); err == nil {
			cmd = exec.Command("pkexec", "pacman", "-S", "--noconfirm", "mongodb")
		} else if _, err := os.Stat("/etc/fedora-release"); err == nil {
			cmd = exec.Command("pkexec", "dnf", "install", "-y", "mongodb")
		} else {
			return fmt.Errorf("unsupported Linux distribution")
		}
	case "darwin":
		cmd = exec.Command("brew", "install", "mongodb-community")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findMongoBin() (string, error) {
	paths := []string{
		"/usr/bin",
		"/usr/local/bin",
		"/opt/homebrew/bin",
		"/opt/homebrew/opt/mongodb-community/bin",
		"/usr/local/mongodb/bin",
	}

	for _, dir := range paths {
		mongod := filepath.Join(dir, "mongod")
		if _, err := os.Stat(mongod); err == nil {
			return dir, nil
		}
	}

	if path, err := exec.LookPath("mongod"); err == nil {
		return filepath.Dir(path), nil
	}

	return "", fmt.Errorf("mongod not found in common locations")
}

func (p *MongoProvider) Name() string { return "mongodb" }

func (p *MongoProvider) Start(ctx context.Context, cfg provider.Config, instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := provider.ValidateConfig(p.Name(), cfg); err != nil {
		return err
	}

	if inst, exists := p.manager.Get(instName); exists {
		if p.IsRunning(inst.DataDir) {
			return fmt.Errorf("instance %q is already running", instName)
		}
		dbPath := filepath.Join(cfg.DataDir, "db")
		logPath := filepath.Join(cfg.DataDir, "mongod.log")
		if err := p.startServer(cfg, dbPath, logPath); err != nil {
			return err
		}
		pid, _ := p.getPID(cfg.DataDir)
		if pid > 0 {
			p.manager.Update(instName, func(i *provider.InstanceInfo) {
				i.PID = pid
			})
		}
		return nil
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "db")
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	logPath := filepath.Join(cfg.DataDir, "mongod.log")

	inst := &provider.InstanceInfo{
		Name:    instName,
		Type:    p.Name(),
		Host:    cfg.Host,
		Port:    cfg.Port,
		DataDir: cfg.DataDir,
	}

	if err := p.manager.Add(inst); err != nil {
		return err
	}

	if err := p.startServer(cfg, dbPath, logPath); err != nil {
		p.manager.Remove(instName)
		return err
	}

	pid, _ := p.getPID(cfg.DataDir)
	if pid > 0 {
		p.manager.Update(instName, func(i *provider.InstanceInfo) {
			i.PID = pid
		})
	}

	return nil
}

func (p *MongoProvider) getPID(dataDir string) (int, error) {
	pidFile := filepath.Join(dataDir, "mongod.lock")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func (p *MongoProvider) startServer(cfg provider.Config, dbPath, logPath string) error {
	mongod := filepath.Join(p.binPath, "mongod")

	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}

	cmd := exec.Command(mongod,
		"--dbpath", dbPath,
		"--port", strconv.Itoa(cfg.Port),
		"--bind_ip", host,
		"--logpath", logPath,
		"--logappend",
		"--fork",
	)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start mongod: %s", string(output))
	}

	return nil
}

func (p *MongoProvider) Stop(instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	inst, exists := p.manager.Get(instName)
	if !exists {
		return fmt.Errorf("instance %q not found", instName)
	}

	pid, err := p.getPID(inst.DataDir)
	if err == nil && pid > 0 {
		process, _ := os.FindProcess(pid)
		if process != nil {
			process.Kill()
		}
	}

	return nil
}

func (p *MongoProvider) Delete(instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	inst, exists := p.manager.Get(instName)
	if !exists {
		return fmt.Errorf("instance %q not found", instName)
	}

	if p.IsRunning(inst.DataDir) {
		p.Stop(instName)
	}

	return p.manager.Remove(instName)
}

func (p *MongoProvider) Status(instName string) (bool, error) {
	inst, exists := p.manager.Get(instName)
	if !exists {
		return false, fmt.Errorf("instance %q not found", instName)
	}

	return p.IsRunning(inst.DataDir), nil
}

func (p *MongoProvider) List() []*provider.InstanceInfo {
	return p.manager.List()
}

func (p *MongoProvider) IsRunning(dataDir string) bool {
	pid, err := p.getPID(dataDir)
	if err != nil {
		return false
	}

	process, _ := os.FindProcess(pid)
	if process == nil {
		return false
	}

	err = process.Signal(nil)
	return err == nil
}

func (p *MongoProvider) GetInstance(instName string) (*provider.InstanceInfo, bool) {
	return p.manager.Get(instName)
}

func (p *MongoProvider) ConnectionString(cfg provider.Config) string {
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	db := cfg.DBName
	if db == "" {
		db = "test"
	}
	return fmt.Sprintf(
		"mongodb://%s:%d/%s",
		host, cfg.Port, db,
	)
}
