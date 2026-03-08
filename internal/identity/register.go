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
	"os/exec"
	"strings"
)

// RegisterEnclaveIdentity creates a SPIFFE entry for a specific enclave tenant.
func RegisterEnclaveIdentity(tenantID string, namespace string) error {
	spiffeID := fmt.Sprintf("spiffe://vecta.io/enclave/%s", tenantID)
	parentID := "spiffe://vecta.io/ns/spire/sa/spire-agent"
	selector := fmt.Sprintf("k8s:ns:%s", namespace)

	fmt.Printf("🛡️  Establishing Identity: %s\n", spiffeID)

	// We use 'kubectl exec' to reach the server.
	// The '-i' flag is omitted for non-interactive automation.
	cmd := exec.Command("kubectl", "exec", "-n", "spire", "spire-server-0", "--",
		"/opt/spire/bin/spire-server", "entry", "create",
		"-parentID", parentID,
		"-spiffeID", spiffeID,
		"-selector", selector)

	out, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(out)
		// Idempotency: If already exists, we are good.
		if strings.Contains(outputStr, "already exists") {
			fmt.Println("ℹ️  Identity already exists. Skipping.")
			return nil
		}
		return fmt.Errorf("SPIRE registration error: %s", outputStr)
	}

	fmt.Println("✅ Identity Registered.")
	return nil
}
