package cmd

import (
	"github.com/rexadb/rexadb/pkg/output"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [instance-name]",
	Short: "Check database instance status",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			cmd.Help()
			return nil
		}

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

		running, _ := p.Status(instName)
		statusStr := "stopped"
		if running {
			statusStr = "running"
		}

		output.Println()
		output.Println("  " + output.Cyan("┌─────────────────────────────────────────────┐"))
		output.Print("  │ ")
		output.Print(output.Bold(inst.Name + " - STATUS"))
		output.Println("                     │")
		output.Println("  " + output.Cyan("└─────────────────────────────────────────────┘"))
		output.Println()
		output.Print("  ")
		output.Print(output.Gray("Type:          "))
		output.Println(inst.Type)
		output.Print("  ")
		output.Print(output.Gray("Port:          "))
		output.Printf("%d\n", inst.Port)
		output.Print("  ")
		output.Print(output.Gray("Data directory:"))
		output.Println(" " + inst.DataDir)
		output.Print("  ")
		output.Print(output.Gray("Status:        "))
		output.Println(output.StatusColor(statusStr))
		output.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
