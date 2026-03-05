package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"github.com/spf13/cobra"
)

var vectaVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the Vecta stack health",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔍 Vecta Deep Verification Status:")
		
		// Map of component names to the exact command to run
		// We use /var/lib/rancher/k3s/server/token to verify k3s is actually readable
		checks := []struct {
			name string
			path string
			args []string
		}{
			{"K3s Core", "/usr/local/bin/k3s", []string{"kubectl", "get", "nodes"}},
			{"Tetragon eBPF", "/usr/local/bin/k3s", []string{"kubectl", "get", "pods", "-n", "kube-system", "-l", "app.kubernetes.io/name=tetragon"}},
			{"vCluster Enclave", "/usr/local/bin/vcluster", []string{"list", "--namespace", "vcluster-agent-enclave"}},
			{"Local Registry", "/usr/local/bin/k3s", []string{"kubectl", "get", "svc", "-n", "vecta-registry"}},
		}

		for _, c := range checks {
			fmt.Printf("--- %-20s ", c.name)
			
			// Manually inject the KUBECONFIG into the command environment
			runCmd := exec.Command(c.path, c.args...)
			runCmd.Env = append(os.Environ(), "KUBECONFIG=/etc/rancher/k3s/k3s.yaml")
			
			output, err := runCmd.CombinedOutput()
			if err != nil {
				fmt.Printf("❌ FAILED\n   [Error]: %v\n   [Output]: %s\n", err, string(output))
			} else {
				fmt.Println("✅ OK")
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(vectaVerifyCmd)
}

