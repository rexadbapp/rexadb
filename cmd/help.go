package cmd

import (
	"github.com/rexadb/rexadb/pkg/output"
	"github.com/spf13/cobra"
)

var helpCmd = &cobra.Command{
	Use:     "tips [command]",
	Short:   "Show help and tips",
	Aliases: []string{"help"},
	Run: func(cmd *cobra.Command, args []string) {
		output.Println()
		output.Println("  " + output.Cyan("rexadb - Quick Help"))
		output.Println()
		output.Println("  " + output.Gray("Start a PostgreSQL database:"))
		output.Println()
		output.Printf("    %s\n", output.Bold("rexadb start postgres"))
		output.Printf("    %s\n", output.Gray("rexadb start postgres mydb --port 5433"))
		output.Println()

		output.Println("  " + output.Cyan("COMMON COMMANDS"))
		output.Println()
		helpRow("rexadb list", "List all instances")
		helpRow("rexadb start", "Start a database")
		helpRow("rexadb stop", "Stop a database")
		helpRow("rexadb status", "Check instance status")
		helpRow("rexadb restart", "Restart a database")
		helpRow("rexadb verify", "Test connectivity")
		helpRow("rexadb logs", "View database logs")
		helpRow("rexadb delete", "Remove an instance")
		output.Println()

		output.Println("  " + output.Cyan("EXAMPLES"))
		output.Println()
		output.Println("  " + output.Gray("Start with custom port:"))
		output.Println()
		output.Printf("    rexadb start postgres mydb -p 5433\n")
		output.Println()
		output.Println("  " + output.Gray("Start with database name:"))
		output.Println()
		output.Printf("    rexadb start postgres dev -n myapp\n")
		output.Println()
		output.Println("  " + output.Gray("Allow network access:"))
		output.Println()
		output.Printf("    rexadb start postgres dev -H 0.0.0.0\n")
		output.Println()
		output.Println("  " + output.Gray("Delete instance and data:"))
		output.Println()
		output.Printf("    rexadb delete dev -f -d\n")
		output.Println()

		output.Println("  " + output.Cyan("GETTING HELP"))
		output.Println()
		output.Printf("    %s\n", output.Gray("rexadb [command] --help"))
		output.Println()
	},
}

func helpRow(cmd, desc string) {
	output.Printf("  %-24s%s\n", cmd, desc)
}

func init() {
	rootCmd.AddCommand(helpCmd)
}
