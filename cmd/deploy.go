package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	agentImage   string
	agentName    string
	auditTime    string
	customPolicy string
	tenantID     string // NEW: Added for Tenant-First isolation
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an agent to the Vecta Enclave via the Management API",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Prepare the Host-Level Workspace
		// Now uses tenantID to ensure policy is placed in the correct hierarchy.
		updateHostPolicy()

		// 2. Prepare the JSON payload for the Orchestrator
		// Added "tenant" to the payload.
		payload := map[string]interface{}{
			"name":           agentName,
			"image":          agentImage,
			"audit_duration": auditTime,
			"tenant":         tenantID,
		}
		jsonData, _ := json.Marshal(payload)

		fmt.Printf("🚀 Deploying Sovereign Agent [%s] for Tenant [%s] with %s Audit phase...\n", agentName, tenantID, auditTime)

		// 3. Call the Vecta Management API (Local Orchestrator)
		resp, err := http.Post("http://localhost:8000/v1/agent/deploy", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Printf("❌ Connection Failure: Vecta Management API is offline: %v\n", err)
			fmt.Println("👉 Hint: Run 'vecta start-server' first.")
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
			fmt.Println("✅ Deployment accepted.")
			fmt.Printf("🛡️  Warden is monitoring at http://localhost:8000/inspect/v1\n")
			// Updated hint to reflect the isolated directory structure.
			fmt.Printf("📊 Discovery manifest will be saved to: /var/vecta/%s/agents/%s/lib/discovered.yaml\n", tenantID, agentName)
		} else {
			fmt.Printf("❌ Deployment Rejected: API returned status %d\n", resp.StatusCode)
		}
	},
}

// updateHostPolicy ensures the /var/vecta/{tenant} workspace is synced with CLI flags.
func updateHostPolicy() {
	// Dynamically calculate the tenant-specific policy path.
	tenantPath := fmt.Sprintf("/var/vecta/%s", tenantID)
	policyFile := fmt.Sprintf("%s/policy/policy.yaml", tenantPath)

	// Ensure the tenant's policy directory exists on the host.
	_ = exec.Command("sudo", "mkdir", "-p", tenantPath+"/policy").Run()

	// If the user provided a custom policy file path, overwrite the active one.
	if customPolicy != "" {
		fmt.Printf("📥 Injecting custom policy from %s...\n", customPolicy)
		_ = exec.Command("sudo", "cp", customPolicy, policyFile).Run()
	} else if _, err := os.Stat(policyFile); os.IsNotExist(err) {
		// If no policy exists for this tenant, seed a default one.
		fmt.Printf("📝 Sealing Default 'Ironclad' Policy for Tenant %s...\n", tenantID)
		defaultPolicy := []byte("audit_duration: \"1m\"\nforbidden_paths:\n  - \"/tmp/vecta\"\n  - \"/etc/shadow\"\nforbidden_sql:\n  - \"DROP TABLE\"\n  - \"DELETE FROM\"\n  - \"execve\"\n  - \"ptrace\"\nallowed_domains:\n  - \"api.openai.com\"\n  - \"github.com\"\n")
		_ = os.WriteFile("/tmp/policy.yaml", defaultPolicy, 0644)
		_ = exec.Command("sudo", "mv", "/tmp/policy.yaml", policyFile).Run()
	}

	// Dynamically update the audit_duration in the YAML file on the host.
	fmt.Printf("⚖️  Setting Enclave Audit Duration: %s\n", auditTime)
	sedCmd := fmt.Sprintf("sudo sed -i 's/audit_duration:.*/audit_duration: \"%s\"/' %s", auditTime, policyFile)
	err := exec.Command("sh", "-c", sedCmd).Run()
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to update audit_duration in %s: %v\n", policyFile, err)
	}

	// Final check: Ensure correct ownership so Warden can read it.
	_ = exec.Command("sudo", "chown", "-R", "root:root", tenantPath).Run()
	_ = exec.Command("sudo", "chmod", "644", policyFile).Run()
}

func init() {
	deployCmd.Flags().StringVarP(&agentImage, "image", "i", "", "The container image URL (required)")
	deployCmd.Flags().StringVarP(&agentName, "name", "n", "vecta-agent", "The name for the agent pod")

	// Requirement: Configurable Tenant ID for isolation.
	deployCmd.Flags().StringVarP(&tenantID, "tenant", "t", "agent-enclave", "The ID of the tenant for isolation")

	// Requirement 1: Configurable Audit Mode Time.
	deployCmd.Flags().StringVarP(&auditTime, "audit-time", "a", "1m", "Duration for the Audit phase (e.g., 30s, 5m)")

	// Flexibility: Allow custom manifest overlay.
	deployCmd.Flags().StringVarP(&customPolicy, "policy", "p", "", "Path to a custom YAML intent manifest")

	deployCmd.MarkFlagRequired("image")
	rootCmd.AddCommand(deployCmd)
}
