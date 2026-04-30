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
			return nil, fmt.Errorf("failed to install postgres: %w\nPlease install manually:\n  Arch:   sudo pacman -S postgresql\n  Ubuntu/Debian: sudo apt install postgresql\n  macOS:  brew install postgresql@18", installErr)
		}
		pgBin, err = findPostgresBin()
		if err != nil {
			return nil, err
		}
	}

	shareDir := findShareDir(pgBin)
	if shareDir == "" {
		fmt.Println("PostgreSQL installation appears corrupted (missing share files). Reinstalling...")
		if reinstallErr := reinstallPostgres(); reinstallErr != nil {
			return nil, fmt.Errorf("postgresql installation is corrupted and could not be auto-repaired.\nPlease run:\n  brew uninstall postgresql@18 && brew install postgresql@18")
		}
		pgBin, err = findPostgresBin()
		if err != nil {
			return nil, err
		}
		shareDir = findShareDir(pgBin)
		if shareDir == "" {
			return nil, fmt.Errorf("postgresql installation is still broken after reinstall")
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
		cmd = exec.Command("brew", "install", "postgresql@18")
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

func reinstallPostgres() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		fmt.Println()
		fmt.Println("  PostgreSQL installation appears incomplete or corrupted.")
		fmt.Println("  Would you like to reinstall it? (y/n)")
		fmt.Println()
		fmt.Print("  > ")
		
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			return fmt.Errorf("user declined reinstall")
		}
		
		fmt.Println("Uninstalling postgresql@18...")
		cmd = exec.Command("brew", "uninstall", "--force", "--formula", "postgresql@18")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		
		fmt.Println("Unlinking conflicting libpq...")
		cmd = exec.Command("brew", "unlink", "libpq")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		
		fmt.Println("Installing postgresql@18...")
		cmd = exec.Command("brew", "install", "postgresql@18")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		installErr := cmd.Run()
		
		if installErr != nil {
			fmt.Println("Installation had issues, trying force link...")
			cmd = exec.Command("brew", "link", "--overwrite", "--force", "postgresql@18")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
		
		return nil
	default:
		return fmt.Errorf("auto-reinstall not supported on this platform. Please reinstall manually.")
	}
}

func findPostgresBin() (string, error) {
	if path, err := exec.LookPath("pg_ctl"); err == nil {
		binDir := filepath.Dir(path)
		if hasAllBinaries(binDir) {
			return binDir, nil
		}
	}

	paths := []string{
		"/usr/lib/postgresql",
		"/usr/bin",
		"/usr/local/bin",
		"/usr/local/opt/postgresql@18/bin",
		"/usr/local/opt/postgresql@17/bin",
		"/usr/local/opt/postgresql@16/bin",
		"/usr/local/opt/postgresql@15/bin",
		"/usr/local/opt/postgresql@14/bin",
		"/opt/homebrew/opt/postgresql@18/bin",
		"/opt/homebrew/opt/postgresql@17/bin",
		"/opt/homebrew/opt/postgresql@16/bin",
		"/opt/homebrew/opt/postgresql@15/bin",
		"/opt/homebrew/opt/postgresql@14/bin",
		"/opt/homebrew/opt/postgresql/bin",
		"/Applications/Postgres.app/Contents/Versions/latest/bin",
	}

	if runtime.GOOS == "windows" {
		paths = []string{
			`C:\Program Files\PostgreSQL`,
			`C:\Program Files (x86)\PostgreSQL`,
		}
	}

	for _, base := range paths {
		if hasAllBinaries(base) {
			return base, nil
		}

		versions, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, v := range versions {
			binPath := filepath.Join(base, v.Name(), "bin")
			if hasAllBinaries(binPath) {
				return binPath, nil
			}
		}
	}

	return "", fmt.Errorf("postgresql not found in common locations")
}

func hasAllBinaries(dir string) bool {
	required := []string{"pg_ctl", "initdb", "postgres"}
	for _, bin := range required {
		if _, err := os.Stat(filepath.Join(dir, bin)); err != nil {
			return false
		}
	}
	return true
}

func findShareDir(binPath string) string {
	possible := []string{
		filepath.Join(filepath.Dir(binPath), "..", "share", "postgresql"),
		filepath.Join(filepath.Dir(binPath), "..", "share"),
		"/usr/share/postgresql",
		"/opt/homebrew/share/postgresql@18",
		"/opt/homebrew/share/postgresql@17",
		"/opt/homebrew/share/postgresql@16",
		"/opt/homebrew/share/postgresql",
		"/usr/local/share/postgresql",
	}

	for _, dir := range possible {
		if _, err := os.Stat(filepath.Join(dir, "postgres.bki")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "pg_hba.conf.sample")); err == nil {
				return dir
			}
		}
	}
	return ""
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
	} else {
		// Always update port in config
		p.updateConfig(cfg)
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

	time.Sleep(2 * time.Second)

	if !p.IsRunning(cfg.DataDir) {
		logData, _ := os.ReadFile(filepath.Join(cfg.DataDir, "postgresql.log"))
		p.manager.Remove(instName)
		return fmt.Errorf("server failed to start properly:\n%s", string(logData))
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
	shareDir := findShareDir(p.binPath)

	args := []string{
		"-D", cfg.DataDir,
		"-U", cfg.User,
		"--no-locale",
		"--encoding", "UTF8",
	}
	if shareDir != "" {
		args = append(args, "-L", shareDir)
	}

	cmd := exec.Command(filepath.Join(p.binPath, "initdb"), args...)
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

	if listenAddr == "0.0.0.0" || listenAddr == "" {
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
	_, err = f.WriteString(fmt.Sprintf("listen_addresses = 'localhost'\n"))
	if err != nil {
		return err
	}
	_, err = f.WriteString("unix_socket_directories = ''\n")
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

func (p *PostgresProvider) updateConfig(cfg provider.Config) error {
	confPath := filepath.Join(cfg.DataDir, "postgresql.conf")
	
	// Read existing config
	data, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}
	
	newContent := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "port ") {
			newContent += "port = " + strconv.Itoa(cfg.Port) + "\n"
		} else {
			newContent += line + "\n"
		}
	}
	
	return os.WriteFile(confPath, []byte(newContent), 0644)
}

func (p *PostgresProvider) startServer(cfg provider.Config) error {
	os.Chmod(cfg.DataDir, 0700)

	// Check what's using the port
	checkPort := exec.Command("lsof", "-sTCP:LISTEN", "-i", ":"+strconv.Itoa(cfg.Port))
	out, _ := checkPort.Output()
	portOutput := string(out)
	if portOutput != "" {
		fmt.Printf("Port %d is in use:\n%s\n", cfg.Port, portOutput)
	}

	// Kill any existing processes on common postgres ports
	exec.Command("sh", "-c", fmt.Sprintf("lsof -sTCP:LISTEN -i :%d -t 2>/dev/null | xargs kill -9 || true", cfg.Port)).Run()
	time.Sleep(1 * time.Second)

	// Remove stale pid and socket files
	os.Remove(filepath.Join(cfg.DataDir, "postmaster.pid"))

	cmd := exec.Command(
		filepath.Join(p.binPath, "pg_ctl"),
		"-D", cfg.DataDir,
		"-l", filepath.Join(cfg.DataDir, "postgresql.log"),
		"start",
		"-w",
		"-t", "60",
	)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PGDATA="+cfg.DataDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_ctl start failed: %s\n%s", err, string(output))
	}

	time.Sleep(1 * time.Second)

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
	pidFile := filepath.Join(dataDir, "postmaster.pid")
	if _, err := os.Stat(pidFile); err == nil {
		data, _ := os.ReadFile(pidFile)
		lines := strings.Split(string(data), "\n")
		if len(lines) > 0 {
			pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
			if err == nil && pid > 0 {
				proc, err := os.FindProcess(pid)
				if err == nil && proc.Pid == pid {
					return true
				}
			}
		}
	}

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
