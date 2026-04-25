package redis

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

type RedisProvider struct {
	binPath string
	manager *Manager
	mu      sync.Mutex
}

func init() {
	provider.Register("redis", NewRedisProvider)
}

func NewRedisProvider() (provider.Provider, error) {
	redisBin, err := findRedisBin()
	if err != nil {
		fmt.Println("Redis not found. Installing...")
		if installErr := installRedis(); installErr != nil {
			return nil, fmt.Errorf("failed to install redis: %w\nPlease install manually:\n  Arch:   sudo pacman -S redis\n  Ubuntu/Debian: sudo apt install redis-server\n  macOS:  brew install redis", installErr)
		}
		redisBin, err = findRedisBin()
		if err != nil {
			return nil, err
		}
	}

	mgr, err := newManager("redis")
	if err != nil {
		return nil, err
	}

	return &RedisProvider{
		binPath: redisBin,
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

func installRedis() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		if _, err := os.Stat("/etc/debian_version"); err == nil {
			cmd = exec.Command("pkexec", "apt", "install", "-y", "redis-server")
		} else if _, err := os.Stat("/etc/arch-release"); err == nil {
			cmd = exec.Command("pkexec", "pacman", "-S", "--noconfirm", "redis")
		} else if _, err := os.Stat("/etc/fedora-release"); err == nil {
			cmd = exec.Command("pkexec", "dnf", "install", "-y", "redis")
		} else {
			return fmt.Errorf("unsupported Linux distribution")
		}
	case "darwin":
		cmd = exec.Command("brew", "install", "redis")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findRedisBin() (string, error) {
	paths := []string{
		"/usr/bin",
		"/usr/local/bin",
		"/opt/homebrew/bin",
		"/opt/homebrew/opt/redis/bin",
	}

	for _, dir := range paths {
		redisServer := filepath.Join(dir, "redis-server")
		if _, err := os.Stat(redisServer); err == nil {
			return dir, nil
		}
	}

	if path, err := exec.LookPath("redis-server"); err == nil {
		return filepath.Dir(path), nil
	}

	return "", fmt.Errorf("redis-server not found in common locations")
}

func (p *RedisProvider) Name() string { return "redis" }

func (p *RedisProvider) Start(ctx context.Context, cfg provider.Config, instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := provider.ValidateConfig(p.Name(), cfg); err != nil {
		return err
	}

	if inst, exists := p.manager.Get(instName); exists {
		if p.IsRunning(inst.DataDir) {
			return fmt.Errorf("instance %q is already running", instName)
		}
		if err := p.startServer(cfg); err != nil {
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
		return fmt.Errorf("failed to create data directory: %w", err)
	}

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

	if err := p.startServer(cfg); err != nil {
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

func (p *RedisProvider) getPID(dataDir string) (int, error) {
	pidFile := filepath.Join(dataDir, "redis.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func (p *RedisProvider) startServer(cfg provider.Config) error {
	redisServer := filepath.Join(p.binPath, "redis-server")

	cmd := exec.Command(redisServer,
		"--port", strconv.Itoa(cfg.Port),
		"--dir", cfg.DataDir,
		"--dbfilename", "dump.rdb",
		"--pidfile", filepath.Join(cfg.DataDir, "redis.pid"),
		"--logfile", filepath.Join(cfg.DataDir, "redis.log"),
	)

	if cfg.Host != "" {
		cmd.Args = append(cmd.Args, "--bind", cfg.Host)
	} else {
		cmd.Args = append(cmd.Args, "--bind", "127.0.0.1")
	}

	cmd.Env = os.Environ()

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start redis-server: %w", err)
	}

	return nil
}

func (p *RedisProvider) stopServer(dataDir string) error {
	pidFile := filepath.Join(dataDir, "redis.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return err
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid > 0 {
		process, _ := os.FindProcess(pid)
		if process != nil {
			process.Kill()
		}
	}
	return nil
}

func (p *RedisProvider) Stop(instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	inst, exists := p.manager.Get(instName)
	if !exists {
		return fmt.Errorf("instance %q not found", instName)
	}

	if err := p.stopServer(inst.DataDir); err != nil {
		return fmt.Errorf("failed to stop: %w", err)
	}

	return nil
}

func (p *RedisProvider) Delete(instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	inst, exists := p.manager.Get(instName)
	if !exists {
		return fmt.Errorf("instance %q not found", instName)
	}

	if p.IsRunning(inst.DataDir) {
		p.stopServer(inst.DataDir)
	}

	return p.manager.Remove(instName)
}

func (p *RedisProvider) Status(instName string) (bool, error) {
	inst, exists := p.manager.Get(instName)
	if !exists {
		return false, fmt.Errorf("instance %q not found", instName)
	}

	return p.IsRunning(inst.DataDir), nil
}

func (p *RedisProvider) List() []*provider.InstanceInfo {
	return p.manager.List()
}

func (p *RedisProvider) IsRunning(dataDir string) bool {
	pidFile := filepath.Join(dataDir, "redis.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
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

func (p *RedisProvider) GetInstance(instName string) (*provider.InstanceInfo, bool) {
	return p.manager.Get(instName)
}

func (p *RedisProvider) ConnectionString(cfg provider.Config) string {
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf(
		"redis://%s:%d",
		host, cfg.Port,
	)
}
