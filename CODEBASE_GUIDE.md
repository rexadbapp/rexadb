# RexaDB Codebase Guide

## 📋 Table of Contents
1. [Project Overview](#project-overview)
2. [Architecture](#architecture)
3. [Directory Structure](#directory-structure)
4. [Core Concepts](#core-concepts)
5. [Package Breakdown](#package-breakdown)
6. [Command System](#command-system)
7. [How It All Works Together](#how-it-all-works-together)

---

## 📌 Project Overview

**RexaDB** is a command-line tool (CLI) for managing local databases without needing Docker or Podman. It allows developers to easily start, stop, list, and manage multiple database instances directly on their machines.

### What It Does:
- ✅ Start database instances (PostgreSQL, MySQL, MariaDB, MongoDB, Redis, SQLite)
- ✅ Stop running instances
- ✅ View status of instances
- ✅ List all registered instances
- ✅ View logs from database instances
- ✅ Self-update capability
- ✅ View connection strings for instances

### Key Dependencies:
- **Cobra**: A Go library for building CLI applications with commands, flags, and arguments
- **Go 1.26.1**: The programming language

---

## 🏗️ Architecture

RexaDB uses a **Plugin Architecture** with the following layers:

```
┌─────────────────────────────────────────┐
│      CLI Layer (cmd/)                   │
│  - Commands (start, stop, list, etc)    │
│  - User Interface & Output               │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│   Provider System (pkg/provider/)       │
│  - Interface that all databases follow  │
│  - Plugin registration system           │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│  Database Implementations               │
│  - PostgreSQL Provider                  │
│  - MySQL Provider                       │
│  - MariaDB Provider                     │
│  - MongoDB Provider                     │
│  - Redis Provider                       │
│  - SQLite Provider                      │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│   Support Utilities                     │
│  - Instance Manager (tracks instances)  │
│  - Output Formatting (colors, UI)       │
│  - Self-Update Checker                  │
└─────────────────────────────────────────┘
```

---

## 📁 Directory Structure

```
rexadb/
├── main.go                 # Entry point - starts the app
├── go.mod                  # Go module definition (dependencies)
├── README.md               # User-facing documentation
├── cmd/                    # CLI Commands
│   ├── root.go            # Main command setup, version, update checker
│   ├── start.go           # "start" command - launches a database
│   ├── stop.go            # "stop" command - stops a database
│   ├── status.go          # "status" command - checks if running
│   ├── list.go            # "list" command - shows all instances
│   ├── logs.go            # "logs" command - displays database logs
│   ├── delete.go          # "delete" command - removes an instance
│   ├── restart.go         # "restart" command - stops then starts
│   ├── update.go          # "update" command - manually check for updates
│   ├── verify.go          # "verify" command - verifies instance setup
│   ├── help.go            # "help" command - shows help text
│   ├── helpers.go         # Shared utility functions
│   └── root.go            # Root command configuration
├── pkg/                    # Reusable packages/libraries
│   ├── instance/          # Instance management
│   │   └── manager.go     # Tracks and persists database instances
│   ├── output/            # UI and output formatting
│   │   └── output.go      # Colors, text formatting, pretty printing
│   ├── provider/          # Database provider system (plugin architecture)
│   │   ├── provider.go    # Interface all databases must implement
│   │   ├── mariadb/       # MariaDB database provider
│   │   ├── mongodb/       # MongoDB database provider
│   │   ├── mysql/         # MySQL database provider
│   │   ├── postgres/      # PostgreSQL database provider
│   │   ├── redis/         # Redis database provider
│   │   └── sqlite/        # SQLite database provider
│   └── selfupdate/        # Auto-update functionality
│       └── selfupdate.go  # Checks for new versions
├── homebrew/              # Installation package for Homebrew
│   └── rexadb.rb         # Homebrew formula
├── release/               # Pre-built binaries for releases
│   ├── rexadb-darwin-amd64    # macOS Intel version
│   ├── rexadb-darwin-arm64    # macOS ARM (Apple Silicon)
│   ├── rexadb-linux-amd64     # Linux version
│   └── checksums.txt          # File integrity verification
└── scripts/               # Build and deployment scripts
    ├── install.sh         # Installation script
    └── release.sh         # Release build script
```

---

## 🧠 Core Concepts

### 1. **Commands (Cobra)**
Go uses a library called **Cobra** to build CLI applications. Each command is a separate operation users can run:

```bash
rexadb start postgres     # runs startCmd
rexadb stop mydb          # runs stopCmd
rexadb list               # runs listCmd
```

Each command:
- Has arguments (things you pass like database type or instance name)
- Has flags (options like `--port 5432`)
- Has a `RunE` function that executes when called

### 2. **Provider Interface**
The **Provider** is an interface that all database implementations must follow. Think of it like a contract:

```go
type Provider interface {
    Name() string                                    // Get database name
    Start(ctx context.Context, cfg Config, ...) error   // Start instance
    Stop(instanceName string) error                 // Stop instance
    Delete(instanceName string) error               // Remove instance
    Status(instanceName string) (bool, error)       // Check if running
    List() []*InstanceInfo                          // List all instances
    GetInstance(name string) (*InstanceInfo, bool)  // Get specific instance
    IsRunning(dataDir string) bool                  // Check if running
    ConnectionString(cfg Config) string             // Get connection details
}
```

Every database provider (PostgreSQL, MySQL, etc.) must implement all these methods. This is how RexaDB supports multiple databases without duplicating code.

### 3. **Instance Manager**
The **Instance Manager** keeps track of all running database instances by:
- Storing instance info in a JSON file at `~/.rexadb/`
- Loading instances on startup
- Adding/removing instances
- Using file locks to prevent race conditions

### 4. **Configuration (Config struct)**
When starting a database, you pass configuration:

```go
type Config struct {
    Port     int       // What port to run on
    Host     string    // What network address
    DataDir  string    // Where to store data
    User     string    // Database username
    Password string    // Database password
    DBName   string    // Database name to create
}
```

### 5. **Output Formatting**
RexaDB uses ANSI color codes to make terminal output pretty:
- 🟢 **Green**: Success messages
- 🔴 **Red**: Errors
- 🟡 **Yellow**: Warnings
- 🔵 **Cyan**: Information/highlights

---

## 📦 Package Breakdown

### **pkg/provider/provider.go** - The Core System
This file defines:
- `Config` struct: Configuration options
- `InstanceInfo` struct: Information about a running instance
- `Provider` interface: What every database provider must implement
- `Registry` system: Maps database names to their providers (PostgreSQL → PostgresProvider, etc)

**Key functions:**
```go
func Register(name string, factory) {}        // Register a new database type
func GetProvider(dbType string) Provider {}   // Get a provider for a database
func GetRegisteredDatabases() []string {}     // List supported databases
```

### **pkg/instance/manager.go** - Instance Tracking
Manages the lifecycle of database instances:

```go
type Instance struct {
    Name    string  // User-friendly name
    Type    string  // postgres, mysql, mongodb, etc
    Host    string  // localhost or IP
    Port    int     // 5432, 3306, etc
    DataDir string  // /path/to/data
    PID     int     // Process ID
}

type Manager struct {
    instances map[string]*Instance  // In-memory storage
    dir string                       // ~/.rexadb directory
}
```

**Key methods:**
```go
Add(inst) error           // Add new instance to tracking
Get(name) *Instance       // Retrieve instance by name
Remove(name) error        // Stop tracking instance
```

### **pkg/output/output.go** - Pretty Printing
Provides formatting functions with ANSI color codes:

```go
Green("text")      // ✅ Success (green bold text)
Red("text")        // ❌ Error (red bold text)
Yellow("text")     // ⚠️ Warning (yellow bold text)
Cyan("text")       // ℹ️ Info (cyan bold text)
Bold("text")       // Bold text without color
StatusColor(str)   // Color based on status (running=green, stopped=red)
```

### **pkg/provider/postgres/provider.go** - Example: PostgreSQL
Shows how a database provider is implemented:

```go
type PostgresProvider struct {
    binPath string              // Path to postgres binary
    manager *Manager            // Instance manager
}

func init() {
    // Register this provider on startup
    provider.Register("postgres", NewPostgresProvider)
}

// Implements all Provider interface methods
func (p *PostgresProvider) Start(ctx, cfg, name) error { ... }
func (p *PostgresProvider) Stop(name string) error { ... }
func (p *PostgresProvider) Status(name string) (bool, error) { ... }
// ... etc
```

The provider:
1. Checks if PostgreSQL is installed on the system
2. Auto-installs if missing
3. Manages starting/stopping instances
4. Tracks running processes
5. Generates connection strings

---

## 🎮 Command System

### **cmd/root.go** - Main Entry Point
Sets up the root command and global options:

```
rexadb [flags] [command] [args]
```

**Flags:**
- `--version` or `-v`: Show version
- `--no-update-check`: Skip checking for updates
- `--help` or `-h`: Show help

Also handles the update checker that runs in the background.

### **cmd/start.go** - Start a Database
```bash
rexadb start [database] [instance-name]
rexadb start postgres                    # Start postgres, auto-name it
rexadb start postgres mydb               # Start postgres, name it "mydb"
rexadb start postgres --port 5433        # Custom port
rexadb start postgres --data-dir /tmp/db # Custom data directory
```

**What it does:**
1. Gets the database provider (e.g., PostgreSQL)
2. Normalizes the config (sets defaults like port 5432)
3. Calls provider's Start method
4. Saves instance info to tracking file

### **cmd/stop.go** - Stop a Database
```bash
rexadb stop mydb    # Stop the instance named "mydb"
```

### **cmd/list.go** - List All Instances
```bash
rexadb list         # Show simple list
rexadb list -a      # Show detailed view with ports, data dirs, etc
```

### **cmd/status.go** - Check Instance Status
```bash
rexadb status mydb  # Show status, port, data directory
```

### **cmd/logs.go** - View Logs
```bash
rexadb logs mydb         # Last 100 lines
rexadb logs mydb -n 50   # Last 50 lines
rexadb logs mydb -f      # Follow (like tail -f)
```

### **cmd/delete.go** - Delete Instance
```bash
rexadb delete mydb  # Remove from tracking
```

### **cmd/restart.go** - Restart Instance
```bash
rexadb restart mydb # Stop then start
```

### **cmd/update.go** - Check for Updates
```bash
rexadb update       # Manually check for new version
```

### **cmd/helpers.go** - Shared Utilities
Contains helper functions used by multiple commands:

```go
func findInstance(name string) (*InstanceInfo, Provider, error)
// Finds an instance and returns its info + provider

func getAllInstances() []*InstanceInfo
// Gets all registered instances across all providers

func normalizeConfig(dbType string, cfg Config) Config
// Sets default values (postgres uses port 5432, mysql uses 3306, etc)

func getLocalIP() string
// Gets the local network IP address
```

---

## 🚀 How It All Works Together

### **Flow: Starting a PostgreSQL Database**

```
User runs: rexadb start postgres mydb

                    ↓

        cmd/root.go
        - Initializes the app
        - Sets up Cobra command system

                    ↓

        cmd/start.go (startCmd runs)
        - Parse arguments: dbType="postgres", instName="mydb"
        - Parse flags: port, host, user, password, etc

                    ↓

        pkg/provider/provider.go
        - GetProvider("postgres")
        - Looks up "postgres" in the registry

                    ↓

        pkg/provider/postgres/provider.go
        - NewPostgresProvider() initializes
        - Checks if postgres binary exists
        - Auto-installs if missing
        - Returns PostgresProvider instance

                    ↓

        PostgresProvider.Start(ctx, config, "mydb")
        - Creates data directory at ~/.rexadb/mydb
        - Runs postgres binary with config
        - Saves process ID (PID)
        - Registers instance in manager

                    ↓

        pkg/instance/manager.go
        - Adds instance to in-memory map
        - Serializes to JSON file at ~/.rexadb/postgres/instances.json

                    ↓

        cmd/output.go
        - Prints success message in green
        - Shows connection details

User can now connect to: localhost:5432
```

### **Flow: Listing All Instances**

```
User runs: rexadb list

            ↓

    cmd/list.go (listCmd runs)
    - Calls getAllInstances()

            ↓

    cmd/helpers.go - getAllInstances()
    - Gets all registered providers
    - Calls provider.List() on each
    - Gathers all InstanceInfo objects
    - Returns sorted list

            ↓

    cmd/list.go
    - Calls checkInstanceStatus() on each
    - Determines if running or stopped

            ↓

    cmd/output.go
    - Prints table/list with colors
    - Green for running, Red for stopped

User sees:
  postgres-mydb       postgres    5432        running
  mysql-api           mysql       3306        stopped
  redis-cache         redis       6379        running
```

### **Flow: Stopping a Database**

```
User runs: rexadb stop mydb

            ↓

    cmd/stop.go (stopCmd runs)
    - Finds instance named "mydb"

            ↓

    cmd/helpers.go - findInstance()
    - Gets instance from all providers
    - Checks all databases for a match

            ↓

    PostgresProvider.Stop("mydb")
    - Gets the process ID (PID) from saved data
    - Kills the process
    - Removes from instance tracking

            ↓

    pkg/instance/manager.go
    - Removes instance from JSON file

            ↓

    cmd/output.go
    - Prints "Instance mydb stopped successfully"

Database is now stopped
```

---

## 🔧 Key Go Concepts Used

### **Interfaces**
```go
// An interface is a set of methods any type can implement
type Provider interface {
    Start(ctx context.Context, cfg Config, name string) error
    Stop(name string) error
}

// PostgreSQL implements this interface
type PostgresProvider struct { ... }
func (p *PostgresProvider) Start(ctx, cfg, name) error { ... }
func (p *PostgresProvider) Stop(name string) error { ... }

// Now PostgresProvider can be used anywhere Provider is needed
var provider Provider = &PostgresProvider{}
provider.Start(ctx, cfg, "mydb")
```

### **Mutex (Thread Safety)**
```go
type Manager struct {
    mu sync.RWMutex              // Prevents race conditions
    instances map[string]*Instance
}

func (m *Manager) Add(inst *Instance) error {
    m.mu.Lock()                  // Lock before modifying
    defer m.mu.Unlock()           // Unlock when done (even if error)
    m.instances[inst.Name] = inst
    return m.save(instances)
}
```

### **JSON Marshaling/Unmarshaling**
```go
// Convert Go struct to JSON
data, _ := json.MarshalIndent(instances, "", "  ")
os.WriteFile("instances.json", data, 0644)

// Convert JSON back to Go struct
var instances []*Instance
json.Unmarshal(data, &instances)
```

### **Goroutines & Channels**
```go
// Run update check in background (doesn't block user)
go func() {
    selfupdate.CheckForUpdate()
}()

// Set timeout so it doesn't hang forever
select {
case <-done:
    // Update check finished
case <-time.After(3 * time.Second):
    // Timeout - continue anyway
}
```

---

## 📊 Data Flow Diagram

```
┌──────────────────┐
│   User Input     │
│  rexadb start    │
│   postgres       │
└────────┬─────────┘
         │
         ▼
┌──────────────────────────────┐
│  cmd/root.go                 │
│  Parse into Cobra command    │
└────────┬─────────────────────┘
         │
         ▼
┌──────────────────────────────┐
│  cmd/start.go                │
│  Extract args & flags        │
└────────┬─────────────────────┘
         │
         ▼
┌──────────────────────────────┐
│  pkg/provider/provider.go    │
│  GetProvider("postgres")     │
└────────┬─────────────────────┘
         │
         ▼
┌──────────────────────────────────┐
│  pkg/provider/postgres/          │
│  PostgresProvider.Start()        │
└────────┬─────────────────────────┘
         │
         ▼
┌──────────────────────────────┐
│  pkg/instance/manager.go     │
│  Save instance to JSON       │
└────────┬─────────────────────┘
         │
         ▼
┌──────────────────────────────┐
│  pkg/output/output.go        │
│  Print success (green)       │
└──────────────────────────────┘
```

---

## 🎯 Summary

**RexaDB** is a well-architected Go CLI tool that:

1. **Uses Cobra** for command-line interface management
2. **Implements a Provider Pattern** for pluggable database support
3. **Manages State** using JSON files and in-memory tracking
4. **Handles Concurrency** safely with mutexes
5. **Auto-Installs** missing database binaries
6. **Provides Beautiful Output** with terminal colors
7. **Supports Self-Updates** with background version checking
8. **Cross-Platform** with precompiled binaries for macOS, Linux, etc.

The codebase demonstrates good Go practices:
- Clear package organization
- Separation of concerns
- Interface-based design
- Error handling
- Concurrency patterns
- User-friendly CLI design

Each database provider follows the same pattern, making it easy to add new databases (MongoDB, MySQL, etc.) without modifying the core command system.
