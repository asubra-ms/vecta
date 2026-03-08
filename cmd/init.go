package cmd

import (
	"fmt"
	"os"
	"os/exec"
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

		// If you do not see "V3" in your terminal, the binary is OLD.
		fmt.Println("🛡️  VECTA BOOTSTRAP V3 (FORCED STABILITY)")

		if forceInit {
			nukeExistingState()
		}

		// 1. Hard Scrub existing state
		// 1. Hard Scrub
		fmt.Println("🧹 Performing Nuclear Scrub of K3s state...")
		_ = exec.Command("sudo", "k3s-uninstall.sh").Run() // Try official uninstaller first
		_ = exec.Command("sudo", "systemctl", "stop", "k3s").Run()

		// Unmount k3s filesystems that often stay locked
		_ = exec.Command("sh", "-c", "sudo umount /var/lib/rancher/k3s/projected").Run()

		// Delete everything
		_ = exec.Command("sudo", "rm", "-rf", "/var/lib/rancher/k3s", "/etc/rancher/k3s", "/run/k3s").Run()
		_ = exec.Command("sudo", "rm", "-rf", "~/.kube/config").Run()

		// 2. Install K3s
		fmt.Println("📦 Step 1: Bootstrapping K3s (Binding to eno1np0)...")
		k3sInstall := "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--flannel-backend=host-gw --flannel-iface=eno1np0 --node-ip=10.31.0.84 --disable traefik --disable servicelb --write-kubeconfig-mode 644' sh -"
		if err := runShell(k3sInstall); err != nil {
			fmt.Printf("❌ K3s bootstrap failed: %v\n", err)
			os.Exit(1)
		}

		// 3. FORCE Systemd to realize K3s exists and is new
		fmt.Println("⚙️  Reloading systemd and forcing K3s restart...")
		exec.Command("sudo", "systemctl", "daemon-reload").Run()
		exec.Command("sudo", "systemctl", "restart", "k3s").Run()

		// 4. THE STABILIZATION LOOP (We MUST see Attempt logs here)
		fmt.Println("⏳ Entering API Stabilization Loop...")
		success := false
		for i := 1; i <= 30; i++ {
			// We check for the default service account - the final sign of a healthy PKI
			check := exec.Command("/usr/local/bin/k3s", "kubectl", "get", "sa", "default", "--insecure-skip-tls-verify")

			if err := check.Run(); err == nil {
				fmt.Println("✅ PKI & API are ONLINE!")
				success = true
				break
			}

			fmt.Printf("   🔍 Attempt %d/30: Waiting for PKI stabilization (x509 handshake)... \n", i)
			time.Sleep(4 * time.Second)
		}

		if !success {
			fmt.Println("❌ API failed to come online. Run 'sudo journalctl -u k3s' for details.")
			os.Exit(1)
		}

		// 5. Sync Kubeconfig NOW
		syncKubeconfig()

		// 6. Setup Registry
		_ = setupRegistry()

		// 7. Deploy SPIRE (Identity)
		fmt.Println("🪪 Step 2: Bootstrapping SPIRE Identity Layer...")
		spireCmds := []string{
			"helm repo add spiffe https://spiffe.github.io/helm-charts/",
			"helm repo update",
			"helm upgrade --install spire-crds spiffe/spire-crds --namespace spire --create-namespace",
			"helm upgrade --install spire spiffe/spire --namespace spire --create-namespace",
		}
		for _, c := range spireCmds {
			_ = runShell(c)
		}

		// 8. Deploy vCluster (Enclave)
		fmt.Println("🏗️  Step 3: Deploying Virtual Enclave...")
		if err := runShell("vcluster create agent-enclave -n vcluster-agent-enclave --connect=false"); err != nil {
			fmt.Printf("❌ vcluster deployment failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n🚀 Vecta Enclave is ONLINE and RUNNING.")
	},
}

func syncKubeconfig() {
	// 1. Identify the real user behind sudo
	targetUser := os.Getenv("SUDO_USER")
	if targetUser == "" {
		targetUser = os.Getenv("USER")
	}

	// 2. Build the correct home directory path
	// Avoid os.UserHomeDir() here because it returns /root under sudo
	homeDir := "/home/" + targetUser
	if targetUser == "root" {
		homeDir = "/root"
	}

	kubeDir := homeDir + "/.kube"
	configPath := kubeDir + "/config"

	fmt.Printf("🔐 Syncing Kubeconfig for user: %s\n", targetUser)

	// 3. Create directory
	_ = exec.Command("sudo", "mkdir", "-p", kubeDir).Run()

	// 4. Copy the fresh K3s config
	_ = exec.Command("sudo", "cp", "/etc/rancher/k3s/k3s.yaml", configPath).Run()

	// 5. Fix ownership using the string name (cleaner for Go/Sudo interactions)
	_ = exec.Command("sudo", "chown", "-R", targetUser+":"+targetUser, kubeDir).Run()
	_ = exec.Command("sudo", "chmod", "600", configPath).Run()
}

func runShell(command string) error {
	c := exec.Command("sh", "-c", command)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func init() {
	vectaInitCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Force a nuclear scrub before initialization")
	rootCmd.AddCommand(vectaInitCmd)
}

func setupRegistry() error {
	fmt.Println("   Initializing Vecta Local Registry on port 5000...")
	if err := exec.Command("sudo", "docker", "inspect", "vecta-registry").Run(); err == nil {
		fmt.Println("   Registry already exists. Skipping.")
		return nil
	}
	cmd := exec.Command("sudo", "docker", "run", "-d", "-p", "5000:5000",
		"--restart", "always", "--name", "vecta-registry", "registry:2")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("registry startup failed: %s", string(out))
	}
	fmt.Println("✅ Local Registry active.")
	return nil
}

func nukeExistingState() {
	fmt.Println("🧨 FORCE MODE: Performing Nuclear Scrub of the host...")

	// 1. Stop Services
	_ = exec.Command("sudo", "systemctl", "stop", "k3s").Run()
	_ = exec.Command("sudo", "k3s-killall.sh").Run()

	// 2. Kill "Ghost" Processes (the ones that block ports and namespaces)
	fmt.Println("   - Terminating orphaned container shims...")
	_ = exec.Command("sudo", "pkill", "-9", "-f", "containerd-shim").Run()
	_ = exec.Command("sudo", "pkill", "-9", "-f", "k3s").Run()

	// 3. Unmount Stubborn Filesystems
	fmt.Println("   - Unmounting K3s virtual filesystems...")
	_ = exec.Command("sh", "-c", "sudo umount -l /var/lib/rancher/k3s/projected").Run()
	_ = exec.Command("sh", "-c", "sudo umount -l /run/k3s/containerd/io.containerd.runtime.v2.task/k8s/*").Run()

	// 4. Wipe Data Directories & Certificates
	fmt.Println("   - Purging all data and PKI state...")
	dirs := []string{"/var/lib/rancher/k3s", "/etc/rancher/k3s", "/run/k3s", "/var/lib/kubelet"}
	for _, dir := range dirs {
		_ = exec.Command("sudo", "rm", "-rf", dir).Run()
	}

	// 5. Reset local kubeconfig
	home, _ := os.UserHomeDir()
	_ = exec.Command("rm", "-rf", home+"/.kube/config").Run()

	fmt.Println("✅ Host is now clean.")
}
