package cmd

import (
	"os"

	"github.com/rexadb/rexadb/pkg/output"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [instance-name]",
	Short: "Show database instance logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instName := args[0]

		inst, _, err := findInstance(instName)
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

		data, err := os.ReadFile(inst.DataDir + "/postgresql.log")
		if err != nil {
			data, err = os.ReadFile(inst.DataDir + "/redis.log")
			if err != nil {
				data, err = os.ReadFile(inst.DataDir + "/mongod.log")
				if err != nil {
					output.Println()
					output.Println(output.Red("No log file found for this database type"))
					output.Println()
					return nil
				}
			}
		}

		output.Print("Logs for ")
		output.Print(output.Bold(instName))
		output.Printf(" (%s):\n", inst.DataDir)
		output.Println()
		output.Println(string(data))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
