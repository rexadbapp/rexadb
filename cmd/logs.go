package cmd

import (
	"os"
	"time"

	"github.com/rexadb/rexadb/pkg/output"
	"github.com/spf13/cobra"
)

var (
	lines int
	follow bool
)

func init() {
	logsCmd.Flags().IntVarP(&lines, "lines", "n", 100, "Number of lines to show")
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
}

var logsCmd = &cobra.Command{
	Use:   "logs [instance-name]",
	Short: "Show database instance logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instName := args[0]

		inst, exists := findInstanceByName(instName)
		if !exists {
			output.Println()
			output.Println(output.Red("Instance \"" + instName + "\" not found"))
			output.Println()
			output.Print("  Run ")
			output.Print(output.Cyan("rexadb list"))
			output.Println(" to see all instances")
			output.Println()
			return nil
		}

		// Try common log file names
		logPaths := []string{
			inst.DataDir + "/postgresql.log",
			inst.DataDir + "/redis.log",
			inst.DataDir + "/mongod.log",
			inst.DataDir + "/mariadb.log",
			inst.DataDir + "/mysqld.log",
			inst.DataDir + "/mongodb.log",
		}

		var logFile string
		for _, p := range logPaths {
			if _, err := os.Stat(p); err == nil {
				logFile = p
				break
			}
		}

		if logFile == "" {
			output.Println()
			output.Println(output.Red("No log file found for this instance"))
			output.Println()
			return nil
		}

		data, err := os.ReadFile(logFile)
		if err != nil {
			output.Println(output.Red("Error reading log: " + err.Error()))
			return nil
		}

		logContent := string(data)

		// Show last N lines if -n specified
		if lines > 0 && lines < 1000 {
			allLines := 0
			for i := range logContent {
				if logContent[i] == '\n' {
					allLines++
				}
			}
			if allLines > lines {
				lineCount := 0
				for i := len(logContent) - 1; i >= 0; i-- {
					if logContent[i] == '\n' {
						lineCount++
						if lineCount >= lines {
							logContent = logContent[i+1:]
							break
						}
					}
				}
			}
		}

		output.Println()
		output.Print(output.Bold("📄 Logs: "))
		output.Printf("%s\n", output.Gray(logFile))
		output.Println()
		output.Print(logContent)

		if !follow {
			return nil
		}

		output.Println()
		output.Println(output.Yellow("Following... (Ctrl+C to exit)"))
		output.Println()

		for {
			newData, err := os.ReadFile(logFile)
			if err != nil {
				break
			}
			newContent := string(newData)
			if len(newContent) > len(logContent) {
				output.Print(newContent[len(logContent):])
				logContent = newContent
			}
			time.Sleep(100 * time.Millisecond)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}