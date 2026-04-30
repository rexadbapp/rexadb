package cmd

import (
	"github.com/rexadb/rexadb/pkg/selfupdate"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update rexadb to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		return selfupdate.Update()
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}