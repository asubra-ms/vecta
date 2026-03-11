package cmd

import (
	"bytes"
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
	Short: "Establish the Vecta Security Enclave",
	Run: func(cmd *cobra.Command, args []string) {
		os.Setenv("KUBECONFIG", "/etc/rancher/k3s/k3s.yaml")
		fmt.Println("🛡️  VECTA AUTOMATED BOOTSTRAP")

		if forceInit {
			nukeExistingState()
		}

		initializeVectaWorkspace()

		// --- Step 1: K3s ---
		fmt.Println("📦 Step 1: Bootstrapping K3s Control Plane...")
		k3sInstall := "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--flannel-backend=host-gw --flannel-iface=eno1np0 --node-ip=10.31.0.84 --disable traefik --disable servicelb --write-kubeconfig-mode 644' sh -"
		if err := runShell(k3sInstall); err != nil {
			fmt.Printf("❌ K3s install failed: %v\n", err)
			os.Exit(1)
		}
		_ = exec.Command("sudo", "systemctl", "restart", "k3s").Run()

		// --- Step 2: API Stabilization ---
		fmt.Println("⏳ Stabilizing System API...")
		for i := 1; i <= 30; i++ {
			if err := exec.Command("/usr/local/bin/k3s", "kubectl", "get", "sa", "default").Run(); err == nil {
				fmt.Println("✅ API Online.")
				syncKubeconfig()
				break
			}
			time.Sleep(2 * time.Second)
		}

		// --- Step 3: Identity Infrastructure ---
		fmt.Println("🪪 Step 2a: Deploying Identity Layer...")
		_ = runShell("/usr/local/bin/k3s kubectl create namespace spire --dry-run=client -o yaml | /usr/local/bin/k3s kubectl apply -f -")
		_ = runShell("helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/")
		_ = runShell("helm repo update")
		_ = runShell("helm upgrade --install spire-crds spiffe/spire-crds --namespace spire --create-namespace")

		// --- Step 4: Image Bridge ---
		fmt.Println("📦 Step 2b: Importing Local Sovereign Images...")
		_ = runShell("sudo docker save localhost:5000/spire-server:clean | sudo /usr/local/bin/k3s ctr -n k8s.io images import -")
		_ = runShell("sudo docker save localhost:5000/spire-agent:clean | sudo /usr/local/bin/k3s ctr -n k8s.io images import -")

		// --- Step 5: Sovereign Overrides ---
		infraPath := "infra/spire-server"

		fmt.Println("📑 Step 2c: Applying Identity Sovereignty Overrides...")

		_ = runShell("/usr/local/bin/k3s kubectl apply -f " + infraPath + "/configmap.yaml")
		_ = runShell("/usr/local/bin/k3s kubectl apply -f " + infraPath + "/spire-server-sovereign.yaml")

		_ = exec.Command("/usr/local/bin/k3s", "kubectl", "delete", "pod", "spire-server-0", "-n", "spire", "--force").Run()

		// --- Step 6: Identity Verification ---
		verifySovereignty()

		// --- Step 7: Virtual Enclave ---
		// --- Step 7: Virtual Enclave ---
		fmt.Println("🏗️  Step 3: Deploying Isolated Virtual Enclave...")
		vclusterNS := "vcluster-agent-enclave"
		vclusterName := "agent-enclave"

		if err := runShell("vcluster create " + vclusterName + " -n " + vclusterNS + " --connect=false"); err != nil {
			fmt.Printf("❌ Enclave deployment failed: %v\n", err)
			os.Exit(1)
		}

		// NEW: Deterministic Identity Mapping
		if err := registerVclusterIdentity(vclusterName, vclusterNS); err != nil {
			fmt.Printf("⚠️  Identity registration warning: %v\n", err)
		}

		fmt.Println("\n🚀 VECTA BOOTSTRAP COMPLETE: Enclave is Secure and Sovereign.")
	},
}

func verifySovereignty() {
	fmt.Println("⏳ Step 2d: Verifying Identity Sovereignty...")

	// We wait for the Pod to be 'Running'. This is the most stable K8s-native check.
	for i := 1; i <= 60; i++ {
		checkCmd := "/usr/local/bin/k3s kubectl get pods -n spire spire-server-0 -o jsonpath='{.status.phase}' 2>/dev/null"
		out, _ := exec.Command("sh", "-c", checkCmd).Output()

		if strings.TrimSpace(string(out)) == "Running" {
			// Once Running, we give SPIRE 10 seconds to generate its initial RSA keys and open its socket
			time.Sleep(10 * time.Second)
			fmt.Println("✅ SPIRE Identity Server is Active.")
			return
		}

		fmt.Printf("   🔍 Attempt %d/60: Waiting for identity container... \r", i)
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n❌ FATAL: Identity Layer failed to start.")
	os.Exit(1)
}

func syncKubeconfig() {
	_ = runShell("mkdir -p $HOME/.kube && sudo cp /etc/rancher/k3s/k3s.yaml $HOME/.kube/config && sudo chown $(id -u):$(id -g) $HOME/.kube/config")
	os.Setenv("KUBECONFIG", os.Getenv("HOME")+"/.kube/config")
}

func runShell(command string) error {
	c := exec.Command("sh", "-c", command)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func nukeExistingState() {
	fmt.Println("🧨 Nuclear Scrub: Purging System State...")
	_ = runShell("sudo k3s-uninstall.sh || true")
	_ = runShell("sudo pkill -9 k3s || true")
	_ = runShell("sudo rm -rf /run/spire/* /var/lib/rancher/k3s/* /etc/rancher/k3s/* /var/lib/spire/*")
}

func initializeVectaWorkspace() {
	_ = runShell("sudo mkdir -p /usr/local/vecta/policy /usr/local/vecta/bin /usr/local/vecta/lib")
}

// Inside vectaInitCmd:

func init() {
	vectaInitCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Force nuclear scrub")
	rootCmd.AddCommand(vectaInitCmd)
}

// MOVE THIS FUNCTION LATER INTO A SEPARATE FILE like register.go or utils.go etc.
//
// registerVclusterIdentity bridges the host SPIRE server to a specific vCluster tenant.
func registerVclusterIdentity(vclusterName string, namespace string) error {
	fmt.Printf("🪪  Mapping Sovereign Identity for enclave: %s\n", vclusterName)

	parentID := "spiffe://vecta.io/node/rtx6000-primary"
	spiffeID := fmt.Sprintf("spiffe://vecta.io/enclave/%s", vclusterName)

	// Step A: Ensure the Parent Node Entry exists (The "Attestation Foundation")
	// We run this with a 'create'—if it fails because it exists, we move on.
	nodeCmd := []string{
		"/usr/local/bin/k3s", "kubectl", "exec", "-n", "spire", "spire-server-0", "--",
		"/opt/spire/bin/spire-server", "entry", "create",
		"-spiffeID", parentID,
		"-selector", "k8s_psat:cluster:default",
		"-selector", "k8s_psat:agent_ns:spire",
		"-node",
	}
	_ = exec.Command(nodeCmd[0], nodeCmd[1:]...).Run()

	// Step B: Create the Tenant Workload Entry
	workloadCmd := []string{
		"/usr/local/bin/k3s", "kubectl", "exec", "-n", "spire", "spire-server-0", "--",
		"/opt/spire/bin/spire-server", "entry", "create",
		"-spiffeID", spiffeID,
		"-parentID", parentID,
		"-selector", fmt.Sprintf("k8s:ns:%s", namespace),
		"-selector", "k8s:sa:default",
	}

	var stderr bytes.Buffer
	cmd := exec.Command(workloadCmd[0], workloadCmd[1:]...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "already exists") {
			fmt.Printf("ℹ️  Identity path %s is already active.\n", spiffeID)
			return nil
		}
		return fmt.Errorf("identity mapping failed: %s", stderr.String())
	}

	fmt.Printf("✅ Identity Linked: %s -> %s\n", spiffeID, parentID)
	return nil
}
