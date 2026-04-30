package cmd

import (
	"os"
	"time"

	"github.com/rexadb/rexadb/pkg/output"
	"github.com/rexadb/rexadb/pkg/selfupdate"
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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if !noUpdateCheck {
			checkUpdate()
		}
	},
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
var noUpdateCheck bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&noUpdateCheck, "no-update-check", false, "Skip update check")
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Show rexadb version")
}

var updateChecked bool

func checkUpdate() {
	if updateChecked {
		return
	}
	updateChecked = true

	ch := make(chan struct{})
	go func() {
		selfupdate.SetVersion(Version)
		hasUpdate, newVer, notes := selfupdate.CheckForUpdate()
		if hasUpdate {
			selfupdate.PrintUpdateNotice(newVer, notes)
		}
		close(ch)
	}()

	select {
	case <-ch:
	case <-time.After(3 * time.Second):
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.Println()
		output.Println(output.Red("Error: " + err.Error()))
		output.Println()
		os.Exit(1)
	}
}
