package cmd

import (
	"fmt"

	"github.com/rexadb/rexadb/pkg/output"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [instance-name]",
	Short: "Stop a database instance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instName := args[0]

		_, p, err := findInstance(getDbTypeFromInstanceName(instName), instName)
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

		output.Print("Stopping ")
		output.Print(output.Bold(instName))
		output.Println("...")
		output.Println()

		if err := p.Stop(instName); err != nil {
			return fmt.Errorf("failed to stop: %w", err)
		}

		output.Print("Instance ")
		output.Print(output.Green(instName))
		output.Println(" stopped successfully")
		output.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
