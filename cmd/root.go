package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vecta",
	Short: "Vecta: A Secure Agentic Platform Enclave",
	Long: `Vecta is a portable security suite designed to provide 
isolated, audited, and enforced execution environments for 
AI agents on heterogeneous GPU hardware.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// runPreFlight checks kernel and hardware compatibility.
// This is used by 'init' but defined here for global access.
func runPreFlight() bool {
	fmt.Println("🔍 Running Vecta pre-flight checks...")
	allPassed := true

	// 1. Check Kernel (6.8+ preferred)
	kOut, _ := exec.Command("uname", "-r").Output()
	fmt.Printf("OS Kernel: %s", string(kOut))

	// 2. Check for BPF LSM
	lsmFile, err := os.Open("/sys/kernel/security/lsm")
	if err == nil {
		scanner := bufio.NewScanner(lsmFile)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "bpf") {
				fmt.Println("✅ BPF LSM: Enabled")
				break
			}
		}
		lsmFile.Close()
	} else {
		fmt.Println("❌ BPF LSM: Disabled or Missing")
		allPassed = false
	}

	// 3. Phase 1 Hardware Discovery (Incremental Addition)
	fmt.Println("🔍 Checking Hardware for Phase 1 Observability...")
	fmt.Printf("CPU Architecture: %s\n", runtime.GOARCH)
	
	gpuOut, _ := exec.Command("lspci").Output()
	gpuLine := strings.ToLower(string(gpuOut))

	if strings.Contains(gpuLine, "nvidia") {
		fmt.Println("📊 GPU: NVIDIA detected. Host-level DCGM monitoring enabled.")
		if err := exec.Command("nvidia-smi", "-L").Run(); err != nil {
			fmt.Println("⚠️  Warning: NVIDIA GPU found but 'nvidia-smi' failed. Check drivers.")
		} else {
			fmt.Println("✅ NVIDIA Drivers: Operational")
		}
	} else if strings.Contains(gpuLine, "amd") || strings.Contains(gpuLine, "advanced micro devices") {
		fmt.Println("📊 GPU: AMD detected. Phase 1 monitoring active.")
	} else {
		fmt.Println("ℹ️  GPU: No dedicated GPU detected. Running in Standard Enclave mode.")
	}

	return allPassed
}

func init() {
	// Global flags can be defined here if needed
	// e.g., rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
}

