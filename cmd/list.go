package cmd

import (
	"fmt"
	"sort"

	"github.com/rexadb/rexadb/pkg/output"
	"github.com/rexadb/rexadb/pkg/provider"
	"github.com/spf13/cobra"
)

var listAll bool

func init() {
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false, "Show all instances including details")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all database instances",
	Long:  `List all registered database instances with their status. Use -a for detailed view.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		allInstances := getAllInstances()

		if len(allInstances) == 0 {
			output.Println()
			output.Println(output.Yellow("No database instances registered"))
			output.Println()
			output.Print("  Quick start: ")
			output.Println(output.Cyan("rexadb start postgres"))
			return nil
		}

		sort.Slice(allInstances, func(i, j int) bool {
			return allInstances[i].Name < allInstances[j].Name
		})

		if listAll {
			renderDetailedList(allInstances)
		} else {
			renderSimpleList(allInstances)
		}
		return nil
	},
}

func checkInstanceStatus(inst *provider.InstanceInfo) bool {
	p, err := provider.GetProvider(inst.Type)
	if err != nil {
		return false
	}
	running, _ := p.Status(inst.Name)
	return running
}

func renderSimpleList(instances []*provider.InstanceInfo) {
	output.Println()
	output.Println("  " + output.Cyan("Instances"))
	output.Println("  " + output.Gray("────────────────────────────────────────────────────"))
	output.Println()

	for _, inst := range instances {
		running := checkInstanceStatus(inst)
		statusStr := output.Red("stopped")
		if running {
			if inst.Type == "sqlite" || inst.Type == "sqlite3" {
				statusStr = output.Cyan("local")
			} else {
				statusStr = output.Green("running")
			}
		}

		portStr := fmt.Sprintf("%d", inst.Port)
		if inst.Port == 0 {
			portStr = "-"
		}

		output.Printf("  %-12s %-8s port %s  %s\n",
			output.Bold(inst.Name),
			output.Gray(inst.Type),
			portStr,
			statusStr)
	}

	output.Println()

	running := 0
	stopped := 0
	for _, inst := range instances {
		if checkInstanceStatus(inst) {
			running++
		} else {
			stopped++
		}
	}
	output.Printf("  %s %d running, %d stopped\n",
		output.Gray("Total:"),
		running,
		stopped)
	output.Println()
}

func renderDetailedList(instances []*provider.InstanceInfo) {
	output.Println()
	output.Println("  " + output.Cyan("Instance Details"))
	output.Println("  " + output.Gray("────────────────────────────────────────────────────"))
	output.Println()

	for i, inst := range instances {
		running := checkInstanceStatus(inst)
		statusStr := output.Red("stopped")
		if running {
			if inst.Type == "sqlite" || inst.Type == "sqlite3" {
				statusStr = output.Cyan("local")
			} else {
				statusStr = output.Green("running")
			}
		}

		portStr := fmt.Sprintf("%d", inst.Port)
		if inst.Port == 0 {
			portStr = "-"
		}

		if i > 0 {
			output.Println()
		}

		output.Print("  ")
		output.Println(output.Bold(inst.Name))
		output.Println("  " + output.Gray("────────────────────────────────"))
		output.Printf("    %-12s %s\n", output.Gray("Type:"), inst.Type)
		output.Printf("    %-12s %s\n", output.Gray("Port:"), portStr)
		output.Printf("    %-12s %s\n", output.Gray("Status:"), statusStr)
		output.Printf("    %-12s %s\n", output.Gray("Data:"), inst.DataDir)
		if inst.PID > 0 {
			output.Printf("    %-12s %d\n", output.Gray("PID:"), inst.PID)
		}
	}

	output.Println()
	running := 0
	stopped := 0
	for _, inst := range instances {
		if checkInstanceStatus(inst) {
			running++
		} else {
			stopped++
		}
	}
	output.Printf("  %s %d total: %s running, %s stopped\n",
		output.Gray("Summary:"),
		len(instances),
		output.Green(fmt.Sprintf("%d", running)),
		output.Red(fmt.Sprintf("%d", stopped)))
	output.Println()
}

func init() {
	rootCmd.AddCommand(listCmd)
}
