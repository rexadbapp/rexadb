package cmd

import (
	"github.com/rexadb/rexadb/pkg/output"
	"github.com/rexadb/rexadb/pkg/provider"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify [instance-name]",
	Short: "Verify database connectivity and show connection info",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instName := args[0]

		inst, p, err := findInstance(instName)
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
		if !running {
			output.Println()
			output.Println(output.Red("Instance \"" + instName + "\" is NOT running"))
			output.Println()
			output.Print("  Run ")
			output.Print(output.Cyan("rexadb start " + instName))
			output.Println(" to start it")
			output.Println()
			return nil
		}

		output.Println()
		output.Print("Instance ")
		output.Print(output.Green(instName))
		output.Println(" is RUNNING")
		output.Println()

		output.Println("  " + output.Cyan("CONNECTION INFO"))
		output.Println("  " + output.Gray("────────────────────────────────────────────────────"))
		output.Println()
		output.Print("  ")
		output.Print(output.Gray("Type:          "))
		output.Println(inst.Type)
		output.Print("  ")
		output.Print(output.Gray("Port:          "))
		output.Printf("%d\n", inst.Port)
		output.Print("  ")
		output.Print(output.Gray("PID:           "))
		output.Printf("%d\n", inst.PID)
		output.Print("  ")
		output.Print(output.Gray("DataDir:       "))
		output.Println(inst.DataDir)
		output.Println()

		cfg := provider.Config{
			Port:    inst.Port,
			Host:    inst.Host,
			DataDir: inst.DataDir,
			DBName:  inst.Name,
		}
		output.Println("  " + output.Green("Connection: "+p.ConnectionString(cfg)))
		output.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}
