package cmd

import (
	"fmt"
	"os"

	"github.com/rexadb/rexadb/pkg/provider/postgres"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [instance-name]",
	Short: "Show database instance logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instName := args[0]

		p, err := postgres.NewPostgresProvider()
		if err != nil {
			return fmt.Errorf("failed to initialize provider: %w", err)
		}

		inst, exists := p.GetInstance(instName)
		if !exists {
			return fmt.Errorf("instance %q not found", instName)
		}

		data, err := os.ReadFile(inst.DataDir + "/postgresql.log")
		if err != nil {
			return fmt.Errorf("failed to read logs: %w", err)
		}

		fmt.Print(string(data))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
