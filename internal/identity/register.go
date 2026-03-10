// Vecta's Identity Layer:
//
// Once the webhook is ready to inject the volumes, we need the logic to Register the workload in SPIRE.
// This ensures that when the sentry sidecar wakes up, it can find a valid identity at
// /run/spire/sockets
//
// This function is called by vecta enclave create cmd.
//

package identity

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RegisterVclusterIdentity matches the call in server.go
func RegisterVclusterIdentity(tenantID string, namespace string) error {
	spiffeID := fmt.Sprintf("spiffe://vecta.io/enclave/%s", tenantID)
	// Using the standard agent parent ID
	parentID := "spiffe://vecta.io/ns/spire/sa/spire-agent"
	selector := fmt.Sprintf("k8s:ns:%s", namespace)

	fmt.Printf("🛡️  Establishing Identity: %s\n", spiffeID)

	// Use absolute path for k3s and ensure KUBECONFIG is set for the exec call
	cmd := exec.Command("/usr/local/bin/k3s", "kubectl", "exec", "-n", "spire", "spire-server-0", "--",
		"/opt/spire/bin/spire-server", "entry", "create",
		"-parentID", parentID,
		"-spiffeID", spiffeID,
		"-selector", selector,
		"-selector", "k8s:sa:default") // Explicitly bind to default SA for enclave agents

	// Inherit system environment to ensure kubectl has cluster access
	cmd.Env = append(os.Environ(), "KUBECONFIG=/etc/rancher/k3s/k3s.yaml")

	out, err := cmd.CombinedOutput()
	outputStr := string(out)

	if err != nil {
		if strings.Contains(outputStr, "already exists") {
			fmt.Println("ℹ️  Identity already exists. Skipping.")
			return nil
		}
		return fmt.Errorf("SPIRE registration error: %s", outputStr)
	}

	fmt.Println("✅ Identity Registered.")
	return nil
}
