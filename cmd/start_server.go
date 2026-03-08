package cmd

import (
	"fmt"
	"vecta/internal/api" // Adjust this to your actual internal path
	"github.com/spf13/cobra"
	"os"
)

var startServerCmd = &cobra.Command{
	Use:   "start-server",
	Short: "Launch the Vecta Management API and Orchestrator",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetString("port")
		
		fmt.Printf("🛡️  Vecta Orchestrator: Initializing Management API on port %s...\n", port)
		
		// This calls your existing Gin server logic
		err := api.StartServer(port)
		if err != nil {
			fmt.Printf("❌ Critical Failure: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	startServerCmd.Flags().StringP("port", "p", "8000", "Port to run the API server on")
	rootCmd.AddCommand(startServerCmd)
}

