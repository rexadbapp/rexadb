package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/rexadb/rexadb/pkg/output"
	"github.com/rexadb/rexadb/pkg/provider"
	"github.com/spf13/cobra"
)

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return ""
}

func normalizeConfig(dbType string, cfg provider.Config) provider.Config {
	switch dbType {
	case "mysql":
		if cfg.Port == 0 || cfg.Port == 5432 {
			cfg.Port = 3306
		}
		if cfg.User == "" || cfg.User == "postgres" {
			cfg.User = "root"
		}
		if cfg.Password == "" || cfg.Password == "postgres" {
			cfg.Password = "root"
		}
	case "mariadb":
		if cfg.Port == 0 || cfg.Port == 5432 {
			cfg.Port = 3306
		}
		if cfg.User == "" || cfg.User == "postgres" {
			cfg.User = "root"
		}
		if cfg.Password == "" || cfg.Password == "postgres" {
			cfg.Password = "root"
		}
	case "redis":
		if cfg.Port == 0 || cfg.Port == 5432 {
			cfg.Port = 6379
		}
	case "mongodb":
		if cfg.Port == 0 || cfg.Port == 5432 {
			cfg.Port = 27017
		}
	case "postgres", "postgresql":
		if cfg.Port == 0 {
			cfg.Port = 5432
		}
		if cfg.User == "" {
			cfg.User = "postgres"
		}
		if cfg.Password == "" {
			cfg.Password = "postgres"
		}
	}
	return cfg
}

var (
	port     int
	host     string
	dataDir  string
	user     string
	password string
	dbName   string
)

func init() {
	startCmd.Flags().IntVarP(&port, "port", "p", 5432, "Port to run the database on")
	startCmd.Flags().StringVarP(&host, "host", "H", "", "Host to bind to (default: localhost, use 0.0.0.0 for network access)")
	startCmd.Flags().StringVarP(&dataDir, "data-dir", "d", "", "Data directory for the database")
	startCmd.Flags().StringVarP(&user, "user", "u", "postgres", "Database user")
	startCmd.Flags().StringVarP(&password, "password", "w", "postgres", "Database password")
	startCmd.Flags().StringVarP(&dbName, "db-name", "n", "", "Database name to create")
}

var startCmd = &cobra.Command{
	Use:   "start [database] [instance-name]",
	Short: "Start a database instance",
	Long:  fmt.Sprintf("Start a database instance. Supported: %v", provider.GetRegisteredDatabases()),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbType := args[0]
		instName := ""
		if len(args) > 1 {
			instName = args[1]
		}

		if inst, exists := findInstanceByName(dbType, dbType); exists {
			dbType = inst.Type
			if instName == "" {
				instName = inst.Name
			}
		}

		if instName == "" {
			instName = dbType + "-default"
		}

		if dataDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			dataDir = filepath.Join(homeDir, ".rexadb", dbType, instName)
		}

		cfg := provider.Config{
			Port:     port,
			Host:     host,
			DataDir:  dataDir,
			User:     user,
			Password: password,
			DBName:   dbName,
		}

		cfg = normalizeConfig(dbType, cfg)

		p, err := provider.GetProvider(dbType)
		if err != nil {
			return err
		}

		output.Println()
		output.Print("Starting ")
		output.Print(output.Cyan(p.Name()))
		output.Print(" instance ")
		output.Print(output.Bold(instName))
		output.Printf(" on port %d...\n", cfg.Port)

		if err := p.Start(cmd.Context(), cfg, instName); err != nil {
			if strings.Contains(err.Error(), "already running") {
				output.Println()
				output.Print(output.Yellow("Instance "))
				output.Print(output.Bold(instName))
				output.Println(output.Yellow(" is already running"))
				output.Println()
				return nil
			}
			return fmt.Errorf("failed to start: %w", err)
		}

		output.Println()
		output.Print("Instance ")
		output.Print(output.Green(instName))
		output.Println(" started successfully!")
		output.Println()

		output.Println("  " + output.Cyan("CONNECTION INFO"))
		output.Println("  " + output.Gray("────────────────────────────────────────────────────"))
		output.Println()

		if host == "0.0.0.0" {
			localIP := getLocalIP()
			if localIP != "" {
				connStr := p.ConnectionString(cfg)
				output.Print("  ")
				output.Print(output.Gray("Local:   "))
				output.Println(connStr)
				output.Print("  ")
				output.Print(output.Gray("Network: "))
				output.Printf("%s://%s:%s@%s:%d/%s\n", dbType, user, password, localIP, port, dbName)
				output.Println()
				output.Println("  " + output.Yellow("Connect from other devices using the Network connection string."))
			} else {
				output.Print("  ")
				output.Print(output.Gray("Connection: "))
				output.Println(p.ConnectionString(cfg))
			}
		} else {
			output.Print("  ")
			output.Print(output.Gray("Connection: "))
			output.Println(p.ConnectionString(cfg))
		}

		output.Print("  ")
		output.Print(output.Gray("Data dir:   "))
		output.Println(output.Gray(dataDir))
		output.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
