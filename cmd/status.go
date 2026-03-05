package cmd

import (
	"fmt"
	"os/exec"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show tenant vCluster status and pod placement",
	Run: func(cmd *cobra.Command, args []string) {
		tenant, _ := cmd.Flags().GetString("tenant")
		fmt.Printf("📊 Vecta Status for Tenant: %s\n", tenant)

		kEnv := "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml && "
		
		// 1. Check vCluster logical status
		fmt.Printf("--- vCluster Control Plane: ")
		err := exec.Command("sh", "-c", "/usr/local/bin/vcluster list | grep "+tenant).Run()
		if err != nil {
			fmt.Println("❌ NOT FOUND")
			return
		}
		fmt.Println("✅ ONLINE")

		// 2. Show physical pod landing (Host View)
		fmt.Println("\n--- Physical Placement (Host Node View):")
		out, _ := exec.Command("sh", "-c", kEnv+"/usr/local/bin/k3s kubectl get pods -n vcluster-"+tenant).Output()
		fmt.Println(string(out))

		// 3. Show logical pod landing (Inside vCluster View)
		fmt.Println("--- Logical Placement (Inside vCluster):")
		vCmd := fmt.Sprintf("/usr/local/bin/vcluster connect %s --namespace vcluster-%s -- /usr/local/bin/k3s kubectl get pods", tenant, tenant)
		vOut, _ := exec.Command("sh", "-c", kEnv+vCmd).Output()
		fmt.Println(string(vOut))
	},
}

func init() {
	statusCmd.Flags().StringP("tenant", "t", "default-enclave", "Tenant name to check")
	rootCmd.AddCommand(statusCmd)
}

