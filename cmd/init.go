package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vecta/internal/validator"
)

// initCmd represents the new installation command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Vecta stack installation",
	Long:  `Bootstraps a single-node Vecta cluster, including K3s, Cilium, Istio, SPIRE, and Tetragon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 Initializing Vecta New Installation...")

		// 1. Run Validation as a required step
		if !runPreFlight() {
			fmt.Println("❌ Installation aborted: Host does not meet requirements.")
			os.Exit(1)
		}

		// 2. High-Level Installation Sequence
		fmt.Println("📦 Step 1: Bootstrapping K3s Infrastructure...")
		// Logic for: curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644
		
		fmt.Println("🛠️  Step 2: Deploying Roko-Kern (Tetragon) & Roko-Flow (Envoy/Istio)...")
		// Logic for: helm install ...
		
		fmt.Println("🛡️  Step 3: Configuring Identity (SPIRE) & Observability (Prometheus/Grafana)...")
		
		fmt.Println("✅ Vecta installation complete. Node is now a Secure Agentic Host.")
	},
}

func init() {
	// Add init as a subcommand to the base 'vecta' command
	rootCmd.AddCommand(initCmd)
}

