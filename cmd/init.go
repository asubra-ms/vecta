package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var forceInit bool

var vectaInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Establish the Vecta Security Enclave with Host-GW Networking",
	Run: func(cmd *cobra.Command, args []string) {
		// FORCE all sub-processes to use the K3s system config
		os.Setenv("KUBECONFIG", "/etc/rancher/k3s/k3s.yaml")

		fmt.Println("🛡️  VECTA BOOTSTRAP V3 (FORCED STABILITY)")

		// --- STEP 0: WORKSPACE & STATE MANAGEMENT ---
		if forceInit {
			nukeExistingState()
		}

		// Establish the Sovereign Root before K3s installation
		initializeVectaWorkspace()

		// 1. Hard Scrub existing K3s state
		fmt.Println("🧹 Preparing host for K3s installation...")
		_ = exec.Command("sudo", "k3s-uninstall.sh").Run()
		_ = exec.Command("sudo", "systemctl", "stop", "k3s").Run()
		_ = exec.Command("sh", "-c", "sudo umount -l /var/lib/rancher/k3s/projected").Run()

		// 2. Install K3s (Optimized for eno1np0 and host-gw)
		fmt.Println("📦 Step 1: Bootstrapping K3s (Binding to eno1np0)...")
		k3sInstall := "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--flannel-backend=host-gw --flannel-iface=eno1np0 --node-ip=10.31.0.84 --disable traefik --disable servicelb --write-kubeconfig-mode 644' sh -"
		if err := runShell(k3sInstall); err != nil {
			fmt.Printf("❌ K3s bootstrap failed: %v\n", err)
			os.Exit(1)
		}

		// 3. Systemd Stabilization
		fmt.Println("   Reloading systemd and forcing K3s restart...")
		exec.Command("sudo", "systemctl", "daemon-reload").Run()
		exec.Command("sudo", "systemctl", "restart", "k3s").Run()

		// 4. API Stabilization Loop
		fmt.Println("⏳ Entering API Stabilization Loop...")
		success := false
		for i := 1; i <= 30; i++ {
			check := exec.Command("/usr/local/bin/k3s", "kubectl", "get", "sa", "default")
			if err := check.Run(); err == nil {
				fmt.Println("✅ PKI & API are ONLINE!")
				success = true
				break
			}
			fmt.Printf("   🔍 Attempt %d/30: Waiting for API (x509 handshake)... \n", i)
			time.Sleep(4 * time.Second)
		}

		if !success {
			fmt.Println("❌ API failed to come online. Check 'sudo journalctl -u k3s'.")
			os.Exit(1)
		}

		// 5. Post-Boot Configuration
		syncKubeconfig()
		initializeBasePolicy()
		_ = setupRegistry()

		// 6. Deploy Identity Layer (SPIRE) - Sovereign Mode Integration
		fmt.Println("🪪 Step 2: Bootstrapping SPIRE Identity Layer (Sovereign Root)...")

		_ = runShell("helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/")
		_ = runShell("helm repo update")
		_ = runShell("helm upgrade --install spire-crds spiffe/spire-crds --namespace spire --create-namespace")

		// 6b. Apply Sovereign Overrides
		infraPath := "bin/infra/spire-server"
		fmt.Printf("   Applying Identity Overrides from %s\n", infraPath)

		_ = runShell("kubectl apply -f " + infraPath + "/configmap.yaml")
		_ = runShell("kubectl apply -f " + infraPath + "/spire-server-sovereign.yaml")

		// 6c. Hard Reset
		fmt.Println("   Forcing Identity Rotation...")
		_ = exec.Command("kubectl", "delete", "pod", "spire-server-0", "-n", "spire", "--force", "--grace-period=0").Run()

		waitForSpireSovereignty()

		// 7. Deploy Virtual Enclave (vCluster)
		fmt.Println("🏗️  Step 3: Deploying Virtual Enclave...")
		if err := runShell("vcluster create agent-enclave -n vcluster-agent-enclave --connect=false"); err != nil {
			fmt.Printf("❌ vcluster deployment failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n🚀 Vecta Enclave is ONLINE and RUNNING.")
	},
}

func waitForSpireSovereignty() {
	fmt.Println("⏳ Waiting for SPIRE to issue vecta.io Root CA...")
	for i := 1; i <= 30; i++ {
		cmd := "/usr/local/bin/k3s kubectl exec -n spire spire-server-0 -c spire-server -- /opt/spire/bin/spire-server bundle show -format spiffe | grep -o 'vecta.io' | head -1"
		out, err := exec.Command("sh", "-c", cmd).Output()

		if err == nil && strings.Contains(string(out), "vecta.io") {
			fmt.Println("✅ Identity Sovereignty Established (vecta.io)")
			return
		}

		fmt.Printf("   🔍 Attempt %d/30: SPIRE is still initializing keys...\n", i)
		time.Sleep(5 * time.Second)
	}
	fmt.Println("❌ Timeout: SPIRE failed to establish the vecta.io trust domain.")
	os.Exit(1)
}

func init() {
	vectaInitCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Force a nuclear scrub before initialization")
	rootCmd.AddCommand(vectaInitCmd)
}

func runShell(command string) error {
	c := exec.Command("sh", "-c", command)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func nukeExistingState() {
	fmt.Println("🧨 Nuclear Scrub: Purging SPIRE and K3s data...")
	_ = runShell("sudo rm -rf /run/spire/* /var/lib/rancher/k3s/storage/*")
	_ = runShell("kubectl delete namespace spire vcluster-agent-enclave --ignore-not-found")
}

func initializeVectaWorkspace() {
	fmt.Println("🏗️  Ensuring Vecta directories exist...")
	_ = runShell("sudo mkdir -p /var/vecta/policy /var/vecta/logs /var/vecta/bin")
}

func syncKubeconfig() {
	fmt.Println("🔄 Syncing Kubeconfig for local user...")
	_ = runShell("mkdir -p $HOME/.kube && sudo cp /etc/rancher/k3s/k3s.yaml $HOME/.kube/config && sudo chown $(id -u):$(id -g) $HOME/.kube/config")
}

func initializeBasePolicy() {
	fmt.Println("📑 Sealing default Ironclad policy...")
	policyPath := "/var/vecta/policy/policy.yaml"
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		_ = runShell("echo 'audit_duration: \"1m\"\nforbidden_paths:\n  - \"/tmp/vecta\"' | sudo tee " + policyPath)
	}
}

func setupRegistry() error {
	if err := exec.Command("sudo", "docker", "inspect", "vecta-registry").Run(); err == nil {
		return nil
	}
	return runShell("sudo docker run -d -p 5000:5000 --restart always --name vecta-registry registry:2")
}

func init() {
	vectaInitCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Force a nuclear scrub before initialization")
	rootCmd.AddCommand(vectaInitCmd)
}
