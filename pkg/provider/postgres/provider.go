package postgres

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

type PostgresProvider struct {
	binPath string
	manager *Manager
	mu      sync.Mutex
}

func init() {
	provider.Register("postgres", NewPostgresProvider)
}

func NewPostgresProvider() (provider.Provider, error) {
	pgBin, err := findPostgresBin()
	if err != nil {
		fmt.Println("PostgreSQL not found. Installing...")
		if installErr := installPostgres(); installErr != nil {
			return nil, fmt.Errorf("failed to install postgres: %w\nPlease install manually:\n  Arch:   sudo pacman -S postgresql\n  Ubuntu/Debian: sudo apt install postgresql\n  macOS:  brew install postgresql", installErr)
		}
		pgBin, err = findPostgresBin()
		if err != nil {
			return nil, err
		}
	}

	mgr, err := newManager("postgres")
	if err != nil {
		return nil, err
	}

	return &PostgresProvider{
		binPath: pgBin,
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

func installPostgres() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		if _, err := os.Stat("/etc/debian_version"); err == nil {
			cmd = exec.Command("pkexec", "apt", "install", "-y", "postgresql")
		} else if _, err := os.Stat("/etc/arch-release"); err == nil {
			cmd = exec.Command("pkexec", "pacman", "-S", "--noconfirm", "postgresql")
		} else if _, err := os.Stat("/etc/fedora-release"); err == nil {
			cmd = exec.Command("pkexec", "dnf", "install", "-y", "postgresql")
		} else {
			return fmt.Errorf("unsupported Linux distribution")
		}
	case "darwin":
		cmd = exec.Command("brew", "install", "postgresql")
	case "windows":
		if _, err := exec.LookPath("choco"); err == nil {
			cmd = exec.Command("choco", "install", "postgresql", "-y")
		} else if _, err := exec.LookPath("scoop"); err == nil {
			cmd = exec.Command("scoop", "install", "postgresql")
		} else {
			return fmt.Errorf("on Windows, install PostgreSQL manually")
		}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findPostgresBin() (string, error) {
	paths := []string{
		"/usr/lib/postgresql",
		"/usr/bin",
		"/usr/local/bin",
		"/usr/local/opt/postgresql@16/bin",
		"/usr/local/opt/postgresql@15/bin",
		"/usr/local/opt/postgresql@14/bin",
		"/opt/homebrew/opt/postgresql@16/bin",
		"/opt/homebrew/opt/postgresql@15/bin",
		"/opt/homebrew/opt/postgresql@14/bin",
		"/usr/local/opt/postgresql/bin",
		"/Applications/Postgres.app/Contents/Versions/latest/bin",
	}

	if runtime.GOOS == "windows" {
		paths = []string{
			`C:\Program Files\PostgreSQL`,
			`C:\Program Files (x86)\PostgreSQL`,
		}
	}

	for _, base := range paths {
		if _, err := os.Stat(filepath.Join(base, "pg_ctl")); err == nil {
			return base, nil
		}

		versions, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, v := range versions {
			binPath := filepath.Join(base, v.Name(), "bin")
			if _, err := os.Stat(filepath.Join(binPath, "pg_ctl")); err == nil {
				return binPath, nil
			}
		}
	}

	if path, err := exec.LookPath("pg_ctl"); err == nil {
		return filepath.Dir(path), nil
	}

	return "", fmt.Errorf("postgresql not found in common locations")
}

func (p *PostgresProvider) Name() string { return "postgres" }

func (p *PostgresProvider) Start(ctx context.Context, cfg provider.Config, instName string) error {
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
		if err := p.setPassword(cfg); err != nil {
			fmt.Printf("Warning: failed to set password: %v\n", err)
		}
		pid, err := p.getPID(cfg.DataDir)
		if err == nil {
			p.manager.Update(instName, func(i *provider.InstanceInfo) {
				i.PID = pid
			})
		}
		return nil
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if _, err := os.Stat(filepath.Join(cfg.DataDir, "postgresql.conf")); os.IsNotExist(err) {
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

	if err := p.setPassword(cfg); err != nil {
		fmt.Printf("Warning: failed to set password: %v\n", err)
	}

	pid, err := p.getPID(cfg.DataDir)
	if err == nil {
		p.manager.Update(instName, func(i *provider.InstanceInfo) {
			i.PID = pid
		})
	}

	return nil
}

func (p *PostgresProvider) getPID(dataDir string) (int, error) {
	pidFile := filepath.Join(dataDir, "postmaster.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 {
		return strconv.Atoi(strings.TrimSpace(lines[0]))
	}
	return 0, fmt.Errorf("pid not found")
}

func (p *PostgresProvider) initDB(cfg provider.Config) error {
	cmd := exec.Command(
		filepath.Join(p.binPath, "initdb"),
		"-D", cfg.DataDir,
		"-U", cfg.User,
		"--no-locale",
		"--encoding", "UTF8",
	)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PGDATA="+cfg.DataDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("initdb failed: %s", string(output))
	}

	listenAddr := cfg.Host
	if listenAddr == "" {
		listenAddr = "localhost"
	}

	confPath := filepath.Join(cfg.DataDir, "postgresql.conf")
	f, err := os.OpenFile(confPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("\nport = %d\n", cfg.Port))
	if err != nil {
		return err
	}
	_, err = f.WriteString(fmt.Sprintf("listen_addresses = '%s'\n", listenAddr))
	if err != nil {
		return err
	}
	_, err = f.WriteString("unix_socket_directories = '" + cfg.DataDir + "'\n")
	if err != nil {
		return err
	}
	_, err = f.WriteString("password_encryption = md5\n")
	if err != nil {
		return err
	}

	return p.configureAuth(cfg)
}

func (p *PostgresProvider) configureAuth(cfg provider.Config) error {
	hbaPath := filepath.Join(cfg.DataDir, "pg_hba.conf")
	f, err := os.OpenFile(hbaPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	hbaEntries := `
host all all 0.0.0.0/0 md5
host all all ::/0 md5
host all all 127.0.0.1/32 md5
host all all ::1/128 md5
`
	_, err = f.WriteString(hbaEntries)
	return err
}

func (p *PostgresProvider) startServer(cfg provider.Config) error {
	if err := os.Chmod(cfg.DataDir, 0700); err != nil {
		return err
	}

	cmd := exec.Command(
		filepath.Join(p.binPath, "pg_ctl"),
		"-D", cfg.DataDir,
		"-l", filepath.Join(cfg.DataDir, "postgresql.log"),
		"start",
	)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PGDATA="+cfg.DataDir)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start pg_ctl: %w", err)
	}

	err = cmd.Wait()

	if cfg.DBName != "" {
		if err := p.createDB(cfg); err != nil {
			p.stopServer(cfg.DataDir)
			return err
		}
	}

	return nil
}

func (p *PostgresProvider) createDB(cfg provider.Config) error {
	cmd := exec.Command(
		filepath.Join(p.binPath, "createdb"),
		"-h", cfg.DataDir,
		"-p", strconv.Itoa(cfg.Port),
		"-U", cfg.User,
		cfg.DBName,
	)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PGPASSWORD="+cfg.Password)
	cmd.Env = append(cmd.Env, "PGUSER="+cfg.User)

	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "already exists") {
		return fmt.Errorf("failed to create database: %s", string(output))
	}
	return nil
}

func (p *PostgresProvider) setPassword(cfg provider.Config) error {
	cmd := exec.Command(
		filepath.Join(p.binPath, "psql"),
		"-h", cfg.DataDir,
		"-p", strconv.Itoa(cfg.Port),
		"-U", cfg.User,
		"-d", "postgres",
		"-c", fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s';", cfg.User, cfg.Password),
	)
	cmd.Env = os.Environ()

	_, err := cmd.CombinedOutput()
	return err
}

func (p *PostgresProvider) stopServer(dataDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		filepath.Join(p.binPath, "pg_ctl"),
		"-D", dataDir,
		"stop",
		"-m", "fast",
		"-w",
	)
	cmd.Env = os.Environ()

	cmd.Run()
	return nil
}

func (p *PostgresProvider) Stop(instName string) error {
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

func (p *PostgresProvider) Delete(instName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	inst, exists := p.manager.Get(instName)
	if !exists {
		return fmt.Errorf("instance %q not found", instName)
	}

	if p.IsRunning(inst.DataDir) {
		if err := p.stopServer(inst.DataDir); err != nil {
			return fmt.Errorf("failed to stop before delete: %w", err)
		}
	}

	return p.manager.Remove(instName)
}

func (p *PostgresProvider) Status(instName string) (bool, error) {
	inst, exists := p.manager.Get(instName)
	if !exists {
		return false, fmt.Errorf("instance %q not found", instName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		filepath.Join(p.binPath, "pg_ctl"),
		"status",
		"-D", inst.DataDir,
	)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "server is running") {
		return false, nil
	}

	return true, nil
}

func (p *PostgresProvider) List() []*provider.InstanceInfo {
	return p.manager.List()
}

func (p *PostgresProvider) IsRunning(dataDir string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		filepath.Join(p.binPath, "pg_ctl"),
		"status",
		"-D", dataDir,
	)
	cmd.Env = os.Environ()

	output, _ := cmd.CombinedOutput()
	return strings.Contains(string(output), "server is running")
}

func (p *PostgresProvider) GetInstance(instName string) (*provider.InstanceInfo, bool) {
	return p.manager.Get(instName)
}

func (p *PostgresProvider) ConnectionString(cfg provider.Config) string {
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	db := cfg.DBName
	if db == "" {
		db = "postgres"
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		cfg.User, cfg.Password, host, cfg.Port, db,
	)
}
