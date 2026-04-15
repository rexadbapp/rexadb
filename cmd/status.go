package cmd

import (
	"fmt"

	"github.com/rexadb/rexadb/pkg/provider/postgres"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [instance-name]",
	Short: "Check database instance status",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := postgres.NewPostgresProvider()
		if err != nil {
			return fmt.Errorf("failed to initialize provider: %w", err)
		}

		if len(args) == 0 {
			instances := p.List()
			if len(instances) == 0 {
				fmt.Println("No database instances registered")
				return nil
			}

			fmt.Println("Registered instances:")
			for _, inst := range instances {
				status := "stopped"
				if p.IsRunning(inst.DataDir) {
					status = "running"
				}
				fmt.Printf("  %s (%s) - port %d, data: %s [%s]\n",
					inst.Name, inst.Type, inst.Port, inst.DataDir, status)
			}
			return nil
		}

		instName := args[0]
		inst, exists := p.GetInstance(instName)
		if !exists {
			return fmt.Errorf("instance %q not found", instName)
		}

		running, _ := p.Status(instName)
		status := "stopped"
		if running {
			status = "running"
		}

		fmt.Printf("Instance %q:\n", inst.Name)
		fmt.Printf("  Type: %s\n", inst.Type)
		fmt.Printf("  Port: %d\n", inst.Port)
		fmt.Printf("  Data directory: %s\n", inst.DataDir)
		fmt.Printf("  Status: %s\n", status)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
