package cmd

import (
	"fmt"

	"github.com/rexadb/rexadb/pkg/output"
	"github.com/rexadb/rexadb/pkg/provider"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [instance-name]",
	Short: "Restart a database instance",
	Long:  `Stop and start a database instance.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instName := args[0]

		inst, p, err := findInstance(getDbTypeFromInstanceName(instName), instName)
		if err != nil {
			output.Println()
			output.Println(output.Red("Instance \"" + instName + "\" not found"))
			output.Println()
			output.Print("  Run ")
			output.Print(output.Cyan("rexadb list"))
			output.Println(" to see all instances")
			output.Println()
			return nil
		}

		output.Println()
		output.Print(output.Yellow("Stopping " + instName + "...\n"))
		if err := p.Stop(instName); err != nil {
			output.Println(output.Yellow("Warning: stop failed: " + err.Error()))
		}

		output.Println()
		output.Print(output.Cyan("Starting " + instName + "...\n"))

		cfg := provider.Config{
			Port:     inst.Port,
			Host:     inst.Host,
			DataDir:  inst.DataDir,
			User:     "postgres",
			Password: "postgres",
			DBName:   "",
		}

		if err := p.Start(cmd.Context(), cfg, instName); err != nil {
			return fmt.Errorf("failed to start: %w", err)
		}

		output.Println()
		output.Print("Instance ")
		output.Print(output.Green(instName))
		output.Println(" restarted successfully")
		output.Println()

		running, _ := p.Status(instName)
		if running {
			output.Printf("  Connection: %s\n", p.ConnectionString(cfg))
		}
		output.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
