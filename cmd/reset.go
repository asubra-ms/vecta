package cmd // THIS MUST BE CMD

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
		exec.Command("systemctl", "stop", "k3s").Run()

		if _, err := os.Stat("/usr/local/bin/k3s-uninstall.sh"); err == nil {
			exec.Command("sh", "-c", "sudo /usr/local/bin/k3s-uninstall.sh").Run()
		}

		// 2. Handle "Resource Busy" Mounts
		unmountKubelet()

		// 3. Purge Filesystem
		fmt.Println("🧹 Purging Vecta directories...")
		paths := []string{"/var/lib/rancher/k3s", "/etc/rancher/k3s", "/var/lib/kubelet", "/var/lib/cni", "/run/k3s", "/run/flannel"}
		for _, path := range paths {
			exec.Command("rm", "-rf", path).Run()
		}

		// 4. Clean Network Interfaces
		fmt.Println("🌐 Cleaning network interfaces...")
		interfaces := []string{"cni0", "flannel.1", "cilium_host", "cilium_net"}
		for _, iface := range interfaces {
			exec.Command("ip", "link", "delete", iface).Run()
		}

		// 5. Flush Iptables
		fmt.Println("🔥 Flushing Vecta firewall rules...")
		exec.Command("iptables", "-F").Run()
		exec.Command("iptables", "-t", "nat", "-F").Run()

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
				exec.Command("umount", "-l", m).Run()
			}
		}
	}
}
