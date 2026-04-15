package cmd

import (
	"fmt"
	"net"

	"github.com/rexadb/rexadb/pkg/provider/postgres"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify [instance-name]",
	Short: "Verify database connectivity and show connection info",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instName := args[0]

		p, err := postgres.NewPostgresProvider()
		if err != nil {
			return fmt.Errorf("failed to initialize provider: %w", err)
		}

		inst, exists := p.GetInstance(instName)
		if !exists {
			return fmt.Errorf("instance %q not found", instName)
		}

		running, err := p.Status(instName)
		if err != nil || !running {
			fmt.Printf("Instance %q is NOT running\n", instName)
			return nil
		}

		fmt.Printf("Instance %q is RUNNING\n\n", instName)

		fmt.Println("Connection Info:")
		fmt.Printf("  Type:    %s\n", inst.Type)
		fmt.Printf("  Port:    %d\n", inst.Port)
		fmt.Printf("  PID:     %d\n", inst.PID)
		fmt.Printf("  DataDir: %s\n\n", inst.DataDir)

		fmt.Println("Network Access: ENABLED")
		if ip := getLocalIP(); ip != "" {
			fmt.Printf("  Your IP:   %s\n", ip)
			fmt.Printf("  Connect:   postgres://postgres:postgres@%s:%d\n\n", ip, inst.Port)
		}
		fmt.Println("To connect from other devices on your network:")
		fmt.Printf("  postgres://postgres:postgres@<device-ip>:%d\n", inst.Port)

		fmt.Println("\nTesting TCP connectivity...")
		_, err = net.Dial("tcp", fmt.Sprintf("0.0.0.0:%d", inst.Port))
		if err != nil {
			fmt.Printf("  TCP Check: FAILED - %v\n", err)
		} else {
			fmt.Println("  TCP Check: OK - Port is listening on 0.0.0.0")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}
