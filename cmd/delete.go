package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rexadb/rexadb/pkg/output"
	"github.com/spf13/cobra"
)

var (
	forceDelete bool
	deleteData  bool
)

func init() {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Force delete without confirmation")
	deleteCmd.Flags().BoolVarP(&deleteData, "data", "d", false, "Also delete data directory")
}

var deleteCmd = &cobra.Command{
	Use:   "delete [instance-name]",
	Short: "Delete a database instance",
	Long: `Delete a database instance from rexadb registry.
Use -f flag to skip confirmation, and -d to also delete data.
Without arguments, shows interactive selection.`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return interactiveDelete()
		}
		return deleteSingle(args[0])
	},
}

func interactiveDelete() error {
	instances := getAllInstances()
	if len(instances) == 0 {
		output.Println()
		output.Println(output.Yellow("No instances to delete"))
		output.Println()
		return nil
	}

	output.Println()
	output.Println(output.Bold("Select instances to delete"))
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
		output.Printf("  %d) %-12s %-8s port %s  %s\n", i+1, inst.Name, output.Gray(inst.Type), portStr, statusStr)
	}
	output.Println()
	output.Print("Enter numbers to delete (e.g. 1,3 or 1-3), or press ")
	output.Print(output.Gray("[Enter]"))
	output.Println(" to cancel:")
	output.Print(output.Cyan("> "))

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		output.Println()
		output.Println("Cancelled")
		output.Println()
		return nil
	}

	var toDelete []int
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			r := strings.Split(part, "-")
			if len(r) == 2 {
				start, _ := strconv.Atoi(strings.TrimSpace(r[0]))
				end, _ := strconv.Atoi(strings.TrimSpace(r[1]))
				for i := start; i <= end; i++ {
					toDelete = append(toDelete, i)
				}
			}
		} else {
			num, _ := strconv.Atoi(part)
			if num > 0 && num <= len(instances) {
				toDelete = append(toDelete, num)
			}
		}
	}

	if len(toDelete) == 0 {
		output.Println()
		output.Println(output.Red("No valid instances selected"))
		output.Println()
		return nil
	}

	output.Println()
	output.Println(output.Bold("The following instances will be deleted:"))
	output.Println()
	for _, num := range toDelete {
		inst := instances[num-1]
		output.Printf("  - %s (%s)\n", output.Bold(inst.Name), output.Gray(inst.Type))
	}
	output.Println()

	if deleteData {
		output.Println(output.Yellow("WARNING: Data directories will be deleted!"))
		output.Println()
	}

	output.Print("Confirm? ")
	output.Print(output.Gray("[y/N] "))
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "y" && confirm != "yes" {
		output.Println()
		output.Println("Cancelled")
		output.Println()
		return nil
	}

	output.Println()
	var failed []string
	for _, num := range toDelete {
		inst := instances[num-1]
		_, p, err := findInstance(inst.Name)
		if err != nil {
			failed = append(failed, inst.Name)
			continue
		}
		if err := p.Delete(inst.Name); err != nil {
			failed = append(failed, inst.Name)
			continue
		}
		if deleteData {
			os.RemoveAll(inst.DataDir)
		}
		output.Printf("  %s %s deleted\n", output.Green("[✓]"), inst.Name)
	}

	output.Println()
	if len(failed) > 0 {
		output.Printf("%s %d failed\n", output.Red("[!]"), len(failed))
	} else {
		output.Println(output.Green("Done!"))
	}
	output.Println()

	return nil
}

func deleteSingle(instName string) error {
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

	if inst.Type != "sqlite" && inst.Type != "sqlite3" {
		running, _ := p.Status(instName)
		if running {
			output.Println()
			output.Println(output.Yellow("Instance \"" + instName + "\" is running. Stop it first?"))
			output.Println()
			output.Print("  Run ")
			output.Print(output.Cyan("rexadb stop " + instName))
			output.Println(" to stop it")
			output.Println()
			return nil
		}
	}

	if !forceDelete {
		output.Println()
		output.Print("Delete instance ")
		output.Print(output.Bold(instName))
		output.Print("? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input != "y" && input != "yes" {
			output.Println()
			output.Println("Cancelled")
			output.Println()
			return nil
		}
	}

	if err := p.Delete(instName); err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	output.Println()
	if deleteData {
		os.RemoveAll(inst.DataDir)
		output.Printf("  %s  %s\n", output.Gray("Data directory:"), output.Gray(inst.DataDir+" (deleted)"))
	} else {
		output.Printf("  %s  %s\n", output.Gray("Data directory preserved:"), output.Gray(inst.DataDir))
	}
	output.Println()
	output.Print("Instance ")
	output.Print(output.Green(instName))
	output.Println(" deleted successfully")
	output.Println()
	return nil
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
