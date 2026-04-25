package mariadb

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

type MariaDBProvider struct {
	binPath string
	manager *Manager
	mu      sync.Mutex
}

func init() {
	provider.Register("mariadb", NewMariaDBProvider)
}

func NewMariaDBProvider() (provider.Provider, error) {
	mariadbBin, err := findMariaDBBin()
	if err != nil {
		fmt.Println("MariaDB not found. Installing...")
		if installErr := installMariaDB(); installErr != nil {
			return nil, fmt.Errorf("failed to install mariadb: %w\nPlease install manually:\n  Arch:   sudo pacman -S mariadb\n  Ubuntu/Debian: sudo apt install mariadb-server\n  macOS:  brew install mariadb", installErr)
		}
		mariadbBin, err = findMariaDBBin()
		if err != nil {
			return nil, err
		}
	}

	mgr, err := newManager("mariadb")
	if err != nil {
		return nil, err
	}

	return &MariaDBProvider{
		binPath: mariadbBin,
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

func installMariaDB() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		if _, err := os.Stat("/etc/debian_version"); err == nil {
			cmd = exec.Command("pkexec", "apt", "install", "-y", "mariadb-server")
		} else if _, err := os.Stat("/etc/arch-release"); err == nil {
			cmd = exec.Command("pkexec", "pacman", "-S", "--noconfirm", "mariadb")
		} else if _, err := os.Stat("/etc/fedora-release"); err == nil {
			cmd = exec.Command("pkexec", "dnf", "install", "-y", "mariadb")
		} else {
			return fmt.Errorf("unsupported Linux distribution")
		}
	case "darwin":
		cmd = exec.Command("brew", "install", "mariadb")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findMariaDBBin() (string, error) {
	paths := []string{
		"/usr/bin",
		"/usr/local/bin",
		"/opt/homebrew/bin",
		"/opt/homebrew/opt/mariadb/bin",
		"/usr/local/mysql/bin",
	}

	for _, dir := range paths {
		mysqld := filepath.Join(dir, "mysqld")
		if _, err := os.Stat(mysqld); err == nil {
			return dir, nil
		}
	}

	if path, err := exec.LookPath("mysqld"); err == nil {
		return filepath.Dir(path), nil
	}

	return "", fmt.Errorf("mysqld not found in common locations")
}

func (p *MariaDBProvider) Name() string { return "mariadb" }

func (p *MariaDBProvider) Start(ctx context.Context, cfg provider.Config, instName string) error {
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

	if _, err := os.Stat(filepath.Join(cfg.DataDir, "my.cnf")); os.IsNotExist(err) {
		if err := p.initDB(cfg); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
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

func (p *MariaDBProvider) getPID(dataDir string) (int, error) {
	pidFile := filepath.Join(dataDir, "mysql.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func (p *MariaDBProvider) initDB(cfg provider.Config) error {
	mysql_install_db := filepath.Join(p.binPath, "mysql_install_db")
	if _, err := os.Stat(mysql_install_db); os.IsNotExist(err) {
		mysql_install_db = "mysql_install_db"
	}

	cmd := exec.Command(mysql_install_db,
		"--datadir="+cfg.DataDir,
		"--basedir="+filepath.Dir(p.binPath),
	)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "already initialized") {
		return fmt.Errorf("mysql_install_db failed: %s", string(output))
	}

	cnfPath := filepath.Join(cfg.DataDir, "my.cnf")
	f, err := os.Create(cnfPath)
	if err != nil {
		return err
	}
	defer f.Close()

	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}

	conf := fmt.Sprintf(`[mysqld]
datadir=%s
port=%d
bind-address=%s
socket=%s/mysql.sock
pid-file=%s/mysql.pid
`, cfg.DataDir, cfg.Port, host, cfg.DataDir, cfg.DataDir)

	_, err = f.WriteString(conf)
	return err
}

func (p *MariaDBProvider) startServer(cfg provider.Config) error {
	mysqld := filepath.Join(p.binPath, "mysqld")

	cmd := exec.Command(mysqld,
		"--defaults-file="+filepath.Join(cfg.DataDir, "my.cnf"),
	)
	cmd.Env = os.Environ()

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start mysqld: %w", err)
	}

	return nil
}

func (p *MariaDBProvider) stopServer(dataDir string) error {
	socket := filepath.Join(dataDir, "mysql.sock")
	cmd := exec.Command(
		filepath.Join(p.binPath, "mysqladmin"),
		"-u", "root",
		"--socket="+socket,
		"shutdown",
	)
	cmd.Env = os.Environ()
	cmd.Run()
	return nil
}

func (p *MariaDBProvider) Stop(instName string) error {
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

func (p *MariaDBProvider) Delete(instName string) error {
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

func (p *MariaDBProvider) Status(instName string) (bool, error) {
	inst, exists := p.manager.Get(instName)
	if !exists {
		return false, fmt.Errorf("instance %q not found", instName)
	}

	return p.IsRunning(inst.DataDir), nil
}

func (p *MariaDBProvider) List() []*provider.InstanceInfo {
	return p.manager.List()
}

func (p *MariaDBProvider) IsRunning(dataDir string) bool {
	pidFile := filepath.Join(dataDir, "mysql.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}

	process, _ := os.FindProcess(pid)
	return process != nil
}

func (p *MariaDBProvider) GetInstance(instName string) (*provider.InstanceInfo, bool) {
	return p.manager.Get(instName)
}

func (p *MariaDBProvider) ConnectionString(cfg provider.Config) string {
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	db := cfg.DBName
	if db == "" {
		db = "mysql"
	}
	return fmt.Sprintf(
		"mysql://%s:%s@%s:%d/%s",
		cfg.User, cfg.Password, host, cfg.Port, db,
	)
}
