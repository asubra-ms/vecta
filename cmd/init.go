package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var vectaInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Establish the Vecta Security Enclave with Host-GW Networking",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🛡️  Vecta Suite: Starting Stable Host Initialization...")

		// 1. Wipe stale state (Nuclear Scrub)
		fmt.Println("🧹 Scrubbing old cluster state...")
		exec.Command("sudo", "systemctl", "stop", "k3s").Run()
		exec.Command("sudo", "rm", "-rf", "/var/lib/rancher/k3s", "/etc/rancher/k3s").Run()

		// 2. Install K3s with Circle-Breaker flags
		fmt.Println("📦 Step 1: Bootstrapping K3s (Host-GW Mode)...")
		k3sInstall := "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--flannel-backend=host-gw --disable traefik --disable servicelb --write-kubeconfig-mode 644' sh -"
		if err := runShell(k3sInstall); err != nil {
			fmt.Printf("❌ K3s bootstrap failed: %v\n", err)
			os.Exit(1)
		}

		// 3. Stabilization Wait & Sync
		fmt.Println("⏳ Waiting for API stabilization...")
		time.Sleep(20 * time.Second)
		syncKubeconfig()

		// 4. Deploy Enclave Components
		fmt.Println("🏗️  Step 2: Deploying Virtual Enclave...")
		runShell("vcluster create agent-enclave -n vcluster-agent-enclave --connect=false")

		fmt.Println("\n🚀 Vecta Enclave is ONLINE and RUNNING.")
	},
}

func syncKubeconfig() {
	home, _ := os.UserHomeDir()
	kubeDir := home + "/.kube"
	os.MkdirAll(kubeDir, 0755)
	exec.Command("sudo", "cp", "/etc/rancher/k3s/k3s.yaml", kubeDir+"/config").Run()
	uid := os.Getuid()
	gid := os.Getgid()
	exec.Command("sudo", "chown", fmt.Sprintf("%d:%d", uid, gid), kubeDir+"/config").Run()
	os.Chmod(kubeDir+"/config", 0600)
}

func runShell(command string) error {
	c := exec.Command("sh", "-c", command)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func init() {
	rootCmd.AddCommand(vectaInitCmd)
}
