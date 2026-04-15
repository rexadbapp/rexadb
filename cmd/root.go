package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rexadb",
	Short: "rexadb - database provisioning for developers",
	Long: `rexadb makes it easy to spin up databases locally without Docker.
Supports PostgreSQL and other databases for local development.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
