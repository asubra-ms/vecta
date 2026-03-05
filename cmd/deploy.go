package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var agentImage string
var agentName string

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an agent to the Vecta Enclave via the Management API",
	Run: func(cmd *cobra.Command, args []string) {
		// Prepare the JSON payload
		payload := map[string]string{
			"name":  agentName,
			"image": agentImage,
		}
		jsonData, _ := json.Marshal(payload)

		fmt.Printf("🚀 Requesting deployment for %s...\n", agentName)

		// Call the local Start-Server API
		resp, err := http.Post("http://localhost:8000/v1/agent/deploy", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Printf("❌ Failed to connect to Vecta API: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusAccepted {
			fmt.Println("✅ Deployment accepted. Agent is being scheduled and audited.")
		} else {
			fmt.Printf("❌ API returned error: %d\n", resp.StatusCode)
		}
	},
}

func init() {
	deployCmd.Flags().StringVarP(&agentImage, "image", "i", "", "The container image URL (required)")
	deployCmd.Flags().StringVarP(&agentName, "name", "n", "vecta-agent", "The name for the agent pod")
	deployCmd.MarkFlagRequired("image")
	rootCmd.AddCommand(deployCmd)
}

