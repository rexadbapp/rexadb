package cmd

import (
	"fmt"

	"github.com/rexadb/rexadb/pkg/provider/postgres"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [instance-name]",
	Short: "Stop a database instance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instName := args[0]

		p, err := postgres.NewPostgresProvider()
		if err != nil {
			return fmt.Errorf("failed to initialize provider: %w", err)
		}

		if err := p.Stop(instName); err != nil {
			return fmt.Errorf("failed to stop: %w", err)
		}

		fmt.Printf("Instance %q stopped successfully\n", instName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
