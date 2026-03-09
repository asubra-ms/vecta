package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Global tenantID is shared with deploy.go within the cmd package.
// We add statusAgentName to allow users to target specific discovery logs.
var statusAgentName string

var vectaStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Enclave health and Tenant details without external tools",
	Run: func(cmd *cobra.Command, args []string) {
		// Use the tenantID flag (defaults to agent-enclave)
		fmt.Printf("📊 Vecta Sovereign Status: Tenant [%s]\n", tenantID)
		fmt.Println(strings.Repeat("-", 50))

		// 1. Host Infrastructure Health (Preserved)
		checkHostHealth()

		// 2. Tenant Enclave Details (Preserved & updated to use tenantID)
		fmt.Println("\n🏰 Tenant Enclave Details:")
		getTenantDetails(tenantID)

		// 3. Intent Discovery Manifest (Updated for isolated paths)
		if statusAgentName != "" {
			fmt.Printf("\n📄 Active Intent Manifest for Agent [%s]:\n", statusAgentName)
			displayDiscoveredIntent(statusAgentName)
		} else {
			fmt.Println("\n📄 Intent Manifest:")
			fmt.Println("   👉 Hint: Use --agent [name] to view specific discovery logs.")
		}
	},
}

func checkHostHealth() {
	// GPU Check (Preserved)
	gpu, err := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu,memory.used", "--format=csv,noheader").Output()
	gpuStatus := "Online"
	if err != nil {
		gpuStatus = "Offline"
	}
	fmt.Printf("🖥️  RTX 6000 GPU: %s [%s]\n", gpuStatus, strings.TrimSpace(string(gpu)))

	// Control Plane Check (Preserved)
	if err := exec.Command("/usr/local/bin/k3s", "kubectl", "get", "nodes").Run(); err == nil {
		fmt.Println("☸️  K3s API:       Healthy")
	} else {
		fmt.Println("❌ K3s API:       Unreachable")
	}
}

func getTenantDetails(tenant string) {
	// The host namespace where vCluster lives
	hostNamespace := "vcluster-" + tenant

	// A. Find the vCluster Control Plane pod (Preserved)
	cpCmd := fmt.Sprintf("/usr/local/bin/k3s kubectl get pods -n %s -l app=vcluster -o jsonpath='{.items[0].status.phase}'", hostNamespace)
	cpStatus, _ := exec.Command("sh", "-c", cpCmd).Output()

	if len(cpStatus) == 0 {
		fmt.Printf("   ❌ Enclave: [%s] NOT FOUND in namespace %s\n", tenant, hostNamespace)
		return
	}
	fmt.Printf("   ✅ Control Plane: %s\n", string(cpStatus))

	// B. List Synced Pods (Preserved)
	fmt.Println("   📦 Synced Virtual Workloads:")
	podQuery := fmt.Sprintf("/usr/local/bin/k3s kubectl get pods -n %s --no-headers", hostNamespace)
	pods, _ := exec.Command("sh", "-c", podQuery).Output()

	lines := strings.Split(string(pods), "\n")
	foundWorkload := false
	for _, line := range lines {
		if strings.Contains(line, "-x-") {
			foundWorkload = true
			parts := strings.Fields(line)
			if len(parts) > 2 {
				rawName := parts[0]
				readableName := strings.Split(rawName, "-x-")[0]

				// Check Warden Logs for state (Preserved)
				state := getWardenState(hostNamespace, rawName)
				fmt.Printf("      > Agent: %-15s | HostPod: %-25s | State: %s\n", readableName, rawName, state)
			}
		}
	}
	if !foundWorkload {
		fmt.Println("      (No virtual workloads deployed in this enclave yet)")
	}
}

func getWardenState(ns, pod string) string {
	// Grep Warden logs (Preserved)
	logCmd := fmt.Sprintf("/usr/local/bin/k3s kubectl logs -n %s %s -c sentry-warden --tail=20", ns, pod)
	out, _ := exec.Command("sh", "-c", logCmd).Output()

	if strings.Contains(string(out), "ENFORCE") {
		return "🔒 ENFORCING"
	}
	return "⏳ AUDITING"
}

func displayDiscoveredIntent(agentName string) {
	// FIXED: Path now correctly maps to the Tenant-First hierarchy
	manifestPath := fmt.Sprintf("/var/vecta/%s/agents/%s/lib/discovered.yaml", tenantID, agentName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Printf("   ❌ No discovery manifest found for agent [%s] in tenant [%s]\n", agentName, tenantID)
		fmt.Printf("      Path searched: %s\n", manifestPath)
		return
	}

	var discovered map[string]interface{}
	if err := yaml.Unmarshal(data, &discovered); err == nil {
		for key := range discovered {
			fmt.Printf("   - [Intent]: %s\n", key)
		}
	}
}

func init() {
	// Use existing tenantID flag variable from deploy.go to avoid "redeclared" error
	vectaStatusCmd.Flags().StringVarP(&tenantID, "tenant", "t", "agent-enclave", "The ID of the tenant to query")

	// Add agent flag specifically for discovery log lookup
	vectaStatusCmd.Flags().StringVarP(&statusAgentName, "agent", "n", "", "Specific agent name to show discovery logs for")

	rootCmd.AddCommand(vectaStatusCmd)
}
