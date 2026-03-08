package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var statusTenant string

var vectaStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the health of GPU, Identity, and Enclave layers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("📊 Vecta Suite: Status for Tenant [%s]\n", statusTenant)
		fmt.Println(strings.Repeat("-", 45))

		// 1. GPU Health (NVIDIA RTX 6000 check)
		fmt.Print("🖥️  GPU (RTX 6000): ")
		gpu, err := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu,memory.used,temperature.gpu", "--format=csv,noheader").Output()
		if err != nil {
			fmt.Println("❌ Driver Error or No GPU Found")
		} else {
			fmt.Printf("Online [%s]\n", strings.TrimSpace(string(gpu)))
		}

		// 2. Control Plane (K3s check)
		fmt.Print("☸️  Control Plane: ")
		k3s := exec.Command("/usr/local/bin/k3s", "kubectl", "get", "nodes")
		if err := k3s.Run(); err == nil {
			fmt.Println("✅ Healthy")
		} else {
			fmt.Println("❌ API Unreachable (Check sudo journalctl -u k3s)")
		}

		// 3. Identity (SPIRE check)
		fmt.Print("🪪  Identity (SPIRE): ")
		spire := exec.Command("/usr/local/bin/k3s", "kubectl", "get", "pods", "-n", "spire", "-l", "app.kubernetes.io/name=spire-server")
		if out, _ := spire.Output(); strings.Contains(string(out), "Running") {
			fmt.Println("✅ Issuing SVIDs")
		} else {
			fmt.Println("⏳ Starting/Missing")
		}

		// 4. Enclave Density (Filtered by Tenant Namespace)
		namespace := "vcluster-" + statusTenant
		fmt.Printf("🛡️  Active Agents [%s]: ", namespace)

		agents, _ := exec.Command("/usr/local/bin/k3s", "kubectl", "get", "pods", "-n", namespace, "--no-headers", "-l", "vecta.io/tenant="+statusTenant).Output()

		lines := strings.Split(strings.TrimSpace(string(agents)), "\n")
		count := 0
		if len(lines) > 0 && lines[0] != "" {
			count = len(lines)
		}
		fmt.Printf("%d agents detected\n", count)
	},
}

func init() {
	// Added the flag exactly as requested
	vectaStatusCmd.Flags().StringVarP(&statusTenant, "tenant", "t", "agent-enclave", "Tenant name to check")
	rootCmd.AddCommand(vectaStatusCmd)
}
