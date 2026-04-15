package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/rexadb/rexadb/pkg/provider"
	"github.com/rexadb/rexadb/pkg/provider/postgres"
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
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbType := args[0]
		instName := ""
		if len(args) > 1 {
			instName = args[1]
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

		var p *postgres.PostgresProvider
		var err error

		switch dbType {
		case "postgres", "postgresql":
			p, err = postgres.NewPostgresProvider()
		default:
			return fmt.Errorf("unsupported database: %s", dbType)
		}

		if err != nil {
			return fmt.Errorf("failed to initialize provider: %w", err)
		}

		fmt.Printf("Starting %s instance %q on port %d...\n", p.Name(), instName, port)

		if err := p.Start(cmd.Context(), cfg, instName); err != nil {
			return fmt.Errorf("failed to start: %w", err)
		}

		fmt.Printf("Instance %q started successfully!\n", instName)

		if host == "0.0.0.0" {
			localIP := getLocalIP()
			if localIP != "" {
				networkConnStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, password, localIP, port, dbName)
				fmt.Printf("Local connection:  %s\n", p.ConnectionString(cfg))
				fmt.Printf("Network connection: %s\n", networkConnStr)
				fmt.Printf("\nConnect from other devices using the Network connection string above.\n")
			} else {
				fmt.Printf("Connection: %s\n", p.ConnectionString(cfg))
			}
		} else {
			fmt.Printf("Connection: %s\n", p.ConnectionString(cfg))
		}
		fmt.Printf("Data directory: %s\n", dataDir)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
