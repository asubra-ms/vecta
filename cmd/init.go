package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time" // Added for the wait loop

	"github.com/spf13/cobra"
)

var vectaInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Establish the Vecta Security Enclave",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🛡️  Vecta Suite: Starting Greenfield Host Initialization...")

		// 1. Pre-flight checks
		if !runPreFlight() {
			fmt.Println("❌ Host does not meet Vecta security requirements.")
			os.Exit(1)
		}

		// 2. Install K3s
		fmt.Println("📦 Step 1: Bootstrapping Host K3s...")
		k3sInstall := "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable traefik --disable servicelb --write-kubeconfig-mode 644' sh -"
		if err := runShell(k3sInstall); err != nil {
			fmt.Printf("❌ K3s bootstrap failed: %v\n", err)
			os.Exit(1)
		}

		// NEW: Critical Wait for K3s API to be ready before proceeding
		waitForK3s()

		// 3. Install Tetragon
		fmt.Println("🛠️  Step 2: Deploying Tetragon Wardens...")
		// Use the K3s config explicitly to avoid 127.0.0.1:8080 errors
		kEnv := "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml && "
		runShell(kEnv + "helm repo add cilium https://charts.cilium.io || true")
		runShell(kEnv + "helm repo update")
		runShell(kEnv + "helm install tetragon cilium/tetragon -n kube-system")

		// 4. Install vCluster
		fmt.Println("🏗️  Step 3: Creating Virtual Enclave (vCluster)...")
		ensureVClusterCLI() // Fixes the 'vcluster not found' error
		setupVCluster()

		// 5. Setup Local Registry
		setupLocalRegistry()

		fmt.Println("\n🚀 Vecta Suite is now READY.")
		fmt.Println("📍 Management API: Run 'make start-server' to begin.")
	},
}

func waitForK3s() {
	fmt.Println("⏳ Waiting for K3s API server to stabilize (max 60s)...")
	kEnv := "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml && "
	for i := 0; i < 12; i++ {
		cmd := exec.Command("sh", "-c", kEnv+"kubectl get nodes")
		if err := cmd.Run(); err == nil {
			fmt.Println("✅ K3s Control Plane is Online.")
			return
		}
		time.Sleep(5 * time.Second)
	}
	fmt.Println("❌ K3s failed to start. Check 'journalctl -u k3s'.")
	os.Exit(1)
}

func ensureVClusterCLI() {
	if _, err := exec.LookPath("vcluster"); err != nil {
		fmt.Println("📥 Installing vCluster CLI...")
		installCmd := "curl -L -o vcluster \"https://github.com/loft-sh/vcluster/releases/latest/download/vcluster-linux-amd64\" && sudo install -c -m 0755 vcluster /usr/local/bin && rm -f vcluster"
		runShell(installCmd)
	}
}

func setupVCluster() {
	kEnv := "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml && "
	runShell(kEnv + "kubectl create namespace vcluster-agent-enclave || true")
	// Use the explicit Kubeconfig for vcluster creation
	vCmd := kEnv + "vcluster create agent-enclave -n vcluster-agent-enclave --connect=false"
	if err := runShell(vCmd); err != nil {
		fmt.Printf("⚠️  vCluster creation warning: %v\n", err)
	} else {
		fmt.Println("✅ vCluster 'agent-enclave' initialized.")
	}
}

func setupLocalRegistry() {
	fmt.Println("📦 Step 4: Initializing Local Vecta Registry (port 30500)...")
	kEnv := "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml && "
	runShell(kEnv + "kubectl create namespace vecta-registry || true")

	registryManifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
  namespace: vecta-registry
spec:
  replicas: 1
  selector:
    matchLabels:
      app: registry
  template:
    metadata:
      labels:
        app: registry
    spec:
      containers:
      - name: registry
        image: registry:2
        ports:
        - containerPort: 5000
---
apiVersion: v1
kind: Service
metadata:
  name: registry-service
  namespace: vecta-registry
spec:
  type: NodePort
  selector:
    app: registry
  ports:
    - port: 5000
      nodePort: 30500
`
	cmd := exec.Command("sh", "-c", kEnv+"kubectl apply -f -")
	cmd.Stdin = strings.NewReader(registryManifest)
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  Local registry setup failed: %v\n", err)
	} else {
		fmt.Println("✅ Local Registry ready at localhost:30500")
	}
}

func runShell(command string) error {
	c := exec.Command("sh", "-c", command)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}


var tenantName string

func init() {
	vectaInitCmd.Flags().StringVarP(&tenantName, "tenant", "t", "default-enclave", "Name of the tenant vCluster")

	rootCmd.AddCommand(vectaInitCmd)
}

