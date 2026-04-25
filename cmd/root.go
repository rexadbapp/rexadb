package cmd

import (
	"os"

	"github.com/rexadb/rexadb/pkg/output"
	"github.com/spf13/cobra"
)

var (
	Version   = "0.1.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "rexadb",
	Short: "rexadb - database provisioning for developers",
	Long: `rexadb makes it easy to spin up databases locally without Docker.
Supports PostgreSQL and other databases for local development.`,
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag || len(args) == 0 {
			output.Println()
			output.Print("  rexadb ")
			output.Printf("v%s\n", output.Green(Version))
			output.Printf("  Build: %s, Commit: %s\n", BuildTime, GitCommit)
			output.Println()
			cmd.Help()
			return
		}
	},
}

var versionFlag bool

func init() {
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Show rexadb version")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.Println()
		output.Println(output.Red("Error: " + err.Error()))
		output.Println()
		os.Exit(1)
	}
}
