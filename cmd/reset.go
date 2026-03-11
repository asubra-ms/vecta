package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var forceReset bool

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Teardown the Vecta stack and return the host to a clean state",
	Run: func(cmd *cobra.Command, args []string) {
		if !forceReset {
			fmt.Print("   This will delete all Vecta enclaves, agents, and local data. Continue? (y/N): ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			if strings.ToLower(scanner.Text()) != "y" {
				fmt.Println("Reset aborted.")
				return
			}
		}

		fmt.Println("🧨 Initializing Vecta Nuclear Reset...")

		// 1. Stop K3s Services
		fmt.Println("🛑 Stopping K3s and Vecta services...")
		_ = exec.Command("sudo", "systemctl", "stop", "k3s").Run()

		if _, err := os.Stat("/usr/local/bin/k3s-uninstall.sh"); err == nil {
			_ = exec.Command("sudo", "sh", "-c", "/usr/local/bin/k3s-uninstall.sh").Run()
		}

		// Ensure any lingering k3s processes are terminated
		_ = exec.Command("sudo", "k3s-killall.sh").Run()

		// 2. Handle "Resource Busy" Mounts
		unmountKubelet()

		// 3. Purge Sovereign Filesystem Hierarchy (Unified Workspace)
		// Requirement: Everything under /var/vecta must be purged for a clean slate.
		fmt.Println("🧹 Purging Vecta Sovereign Root (/usr/local/vecta/bin)...")
		_ = exec.Command("sudo", "rm", "-rf", "/usr/local/vecta/bin").Run()

		// 4. Purge Standard Host Infrastructure Paths
		fmt.Println("🧹 Purging system directories...")
		paths := []string{
			"/var/lib/rancher/k3s",
			"/etc/rancher/k3s",
			"/var/lib/kubelet",
			"/var/lib/cni",
			"/run/k3s",
			"/run/flannel",
			"/run/spire", // Identity sockets
		}
		for _, path := range paths {
			_ = exec.Command("sudo", "rm", "-rf", path).Run()
		}

		// 5. Clean Network Interfaces
		fmt.Println("🌐 Cleaning network interfaces...")
		interfaces := []string{"cni0", "flannel.1", "cilium_host", "cilium_net"}
		for _, iface := range interfaces {
			_ = exec.Command("sudo", "ip", "link", "delete", iface).Run()
		}

		// 6. Flush Firewall (Iptables)
		fmt.Println("🔥 Flushing Vecta firewall rules...")
		_ = exec.Command("sudo", "iptables", "-F").Run()
		_ = exec.Command("sudo", "iptables", "-t", "nat", "-F").Run()

		// 7. Remove Local Docker Registry
		fmt.Println("📦 Removing Vecta local registry...")
		_ = exec.Command("sudo", "docker", "stop", "vecta-registry").Run()
		_ = exec.Command("sudo", "docker", "rm", "-v", "vecta-registry").Run()

		// 8. Clean Local User Kubeconfig
		targetUser := os.Getenv("SUDO_USER")
		if targetUser == "" {
			targetUser = os.Getenv("USER")
		}
		homeDir := "/home/" + targetUser
		if targetUser == "root" {
			homeDir = "/root"
		}
		_ = exec.Command("rm", "-rf", homeDir+"/.kube/config").Run()

		fmt.Println("\n✨ Host is now clean. You can run 'vecta init' to start fresh.")
	},
}

func init() {
	resetCmd.Flags().BoolVarP(&forceReset, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(resetCmd)
}

func unmountKubelet() {
	fmt.Println("🔓 Detaching busy volume and container mounts...")

	// This catches Kubelet volumes, containerd shm, and rootfs mounts
	patterns := []string{"/var/lib/kubelet", "/run/k3s", "/run/containerd"}

	for _, pattern := range patterns {
		cmdStr := fmt.Sprintf("cat /proc/mounts | grep %s | awk '{print $2}'", pattern)
		out, _ := exec.Command("sh", "-c", cmdStr).Output()
		mounts := strings.Split(string(out), "\n")

		for _, m := range mounts {
			if m != "" {
				// -l is MNT_DETACH (lazy unmount), crucial for "resource busy"
				_ = exec.Command("sudo", "umount", "-l", m).Run()
			}
		}
	}
}
