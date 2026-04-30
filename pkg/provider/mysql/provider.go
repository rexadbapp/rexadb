package mysql

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
	"time"

	"github.com/rexadb/rexadb/pkg/provider"
)

type MysqlProvider struct {
	binPath string
	manager *Manager
	mu      sync.Mutex
}

func init() {
	provider.Register("mysql", NewMysqlProvider)
}

func NewMysqlProvider() (provider.Provider, error) {
	mysqlBin, err := findMysqlBin()
	if err != nil {
		return nil, err
	}

	mgr, err := newManager("mysql")
	if err != nil {
		return nil, err
	}

	return &MysqlProvider{
		binPath: mysqlBin,
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

func installMysql() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		if _, err := os.Stat("/etc/debian_version"); err == nil {
			cmd = exec.Command("pkexec", "apt", "install", "-y", "mysql-server")
		} else if _, err := os.Stat("/etc/arch-release"); err == nil {
			cmd = exec.Command("pkexec", "pacman", "-S", "--noconfirm", "mariadb")
		} else if _, err := os.Stat("/etc/fedora-release"); err == nil {
			cmd = exec.Command("pkexec", "dnf", "install", "-y", "mysql-server")
		} else {
			return fmt.Errorf("unsupported Linux distribution")
		}
	case "darwin":
		cmd = exec.Command("brew", "install", "mysql")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findMysqlBin() (string, error) {
	paths := []string{
		"/usr/bin",
		"/usr/local/bin",
		"/usr/sbin",
		"/opt/homebrew/bin",
		"/opt/homebrew/opt/mysql/bin",
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

func (p *MysqlProvider) Name() string { return "mysql" }

func (p *MysqlProvider) Start(ctx context.Context, cfg provider.Config, instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cfg.Port == 0 || cfg.Port == 5432 {
		cfg.Port = 3306
	}
	if cfg.User == "" || cfg.User == "postgres" {
		cfg.User = "root"
	}
	if cfg.Password == "" || cfg.Password == "postgres" {
		cfg.Password = "root"
	}

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
		time.Sleep(500 * time.Millisecond)
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

	if err := p.setupDataDir(cfg); err != nil {
		return fmt.Errorf("failed to setup data directory: %w", err)
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

	time.Sleep(500 * time.Millisecond)

	pid, _ := p.getPID(cfg.DataDir)
	if pid > 0 {
		p.manager.Update(instName, func(i *provider.InstanceInfo) {
			i.PID = pid
		})
	}

	return nil
}

func (p *MysqlProvider) setupDataDir(cfg provider.Config) error {
	socketPath := filepath.Join(cfg.DataDir, "mysql.sock")

	if _, err := os.Stat(filepath.Join(cfg.DataDir, "my.cnf")); os.IsNotExist(err) {
		conf := fmt.Sprintf(`[mysqld]
datadir=%s
port=%d
bind-address=0.0.0.0
socket=%s
pid-file=%s/mysqld.pid
`, cfg.DataDir, cfg.Port, socketPath, cfg.DataDir)

		if err := os.WriteFile(filepath.Join(cfg.DataDir, "my.cnf"), []byte(conf), 0644); err != nil {
			return err
		}
	}

	if _, err := os.Stat(filepath.Join(cfg.DataDir, "mysql")); os.IsNotExist(err) {
		installCmd := exec.Command("mariadb-install-db",
			"--datadir="+cfg.DataDir,
			"--auth-root-authentication-method=normal",
		)
		installCmd.Env = os.Environ()
		installCmd.CombinedOutput()

		mysqld := filepath.Join(p.binPath, "mysqld")
		startCmd := exec.Command(mysqld, "--defaults-file="+filepath.Join(cfg.DataDir, "my.cnf"))
		startCmd.Env = os.Environ()
		if err := startCmd.Start(); err != nil {
			return fmt.Errorf("failed to start mysqld: %w", err)
		}

		time.Sleep(2 * time.Second)

		user := cfg.User
		if user == "" {
			user = "root"
		}
		password := cfg.Password
		if password == "" {
			password = "root"
		}

		exec.Command(filepath.Join(p.binPath, "mysql"),
			"--socket="+socketPath,
			"-u", "root",
			"-e", "ALTER USER 'root'@'localhost' IDENTIFIED BY '"+password+"'; ALTER USER 'root'@'127.0.0.1' IDENTIFIED BY '"+password+"'; CREATE USER IF NOT EXISTS '"+user+"'@'%' IDENTIFIED BY '"+password+"'; GRANT ALL PRIVILEGES ON *.* TO '"+user+"'@'%' WITH GRANT OPTION; FLUSH PRIVILEGES;",
		).Run()

		exec.Command(filepath.Join(p.binPath, "mysqladmin"),
			"--socket="+socketPath,
			"-u", "root",
			"-p"+password,
			"shutdown",
		).Run()
	}

	return nil
}

func (p *MysqlProvider) getPID(dataDir string) (int, error) {
	pidFile := filepath.Join(dataDir, "mysqld.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func (p *MysqlProvider) startServer(cfg provider.Config) error {
	mysqld := filepath.Join(p.binPath, "mysqld")

	cmd := exec.Command(mysqld,
		"--defaults-file="+filepath.Join(cfg.DataDir, "my.cnf"),
		"--user="+cfg.User,
	)
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mysqld: %w", err)
	}

	return nil
}

func (p *MysqlProvider) Stop(instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	inst, exists := p.manager.Get(instName)
	if !exists {
		return fmt.Errorf("instance %q not found", instName)
	}

	pid, _ := p.getPID(inst.DataDir)
	if pid > 0 {
		process, _ := os.FindProcess(pid)
		if process != nil {
			process.Kill()
		}
	}

	return nil
}

func (p *MysqlProvider) Delete(instName string) error {
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

func (p *MysqlProvider) Status(instName string) (bool, error) {
	inst, exists := p.manager.Get(instName)
	if !exists {
		return false, fmt.Errorf("instance %q not found", instName)
	}

	return p.IsRunning(inst.DataDir), nil
}

func (p *MysqlProvider) List() []*provider.InstanceInfo {
	return p.manager.List()
}

func (p *MysqlProvider) IsRunning(dataDir string) bool {
	pidFile := filepath.Join(dataDir, "mysqld.pid")
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return false
	}

	pid, err := p.getPID(dataDir)
	if err != nil || pid == 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Pid == pid
}

func (p *MysqlProvider) GetInstance(instName string) (*provider.InstanceInfo, bool) {
	return p.manager.Get(instName)
}

func (p *MysqlProvider) ConnectionString(cfg provider.Config) string {
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	user := cfg.User
	if user == "" {
		user = "root"
	}
	password := cfg.Password
	if password == "" {
		password = "root"
	}
	db := cfg.DBName
	if db == "" {
		db = "mysql"
	}
	return fmt.Sprintf(
		"mysql://%s:%s@%s:%d/%s",
		user, password, host, cfg.Port, db,
	)
}
