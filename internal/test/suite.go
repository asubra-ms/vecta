package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type TestCase struct {
	Name string
	Run  func() error
}

// 2. Deployment & Landing Check
func RunSanity() error {
	// 1. Check if the Host Cluster is actually responding
	// This catches the "Connection Refused" issue immediately
	k3sCheck := exec.Command("sh", "-c", "KUBECONFIG=/etc/rancher/k3s/k3s.yaml /usr/local/bin/k3s kubectl get nodes")
	if err := k3sCheck.Run(); err != nil {
		return fmt.Errorf("Host K3s API is OFFLINE (Connection Refused). Run 'vecta reset' and 'vecta init'")
	}

	// 2. Check if Vecta Management API is responding
	resp, err := http.Get("http://localhost:8000/status")
	if err != nil || resp.StatusCode != 200 {
		return fmt.Errorf("Vecta Management API is unreachable")
	}
	return nil
}

func RunLandingTest(tenant, agentName string) error {
	// Trigger API
	payload, _ := json.Marshal(map[string]string{
		"name":   agentName,
		"image":  "alpine",
		"tenant": tenant,
	})

	resp, err := http.Post("http://localhost:8000/v1/agent/deploy", "application/json", bytes.NewBuffer(payload))
	if err != nil || resp.StatusCode != 202 { // 202 is StatusAccepted from your code
		return fmt.Errorf("deployment API failed")
	}

	fmt.Println("   ⏳ Waiting for vCluster sync...")
	time.Sleep(7 * time.Second)

	// Verify Physical vs Logical Landing
	k3s := "/usr/local/bin/k3s"
	vcluster := "/usr/local/bin/vcluster"
	kEnv := "KUBECONFIG=/etc/rancher/k3s/k3s.yaml"

	// Check if pod exists inside the virtual cluster
	virtCmd := fmt.Sprintf("%s %s connect %s --namespace vcluster-%s -- %s kubectl get pods", kEnv, vcluster, tenant, tenant, k3s)
	out, _ := exec.Command("sh", "-c", virtCmd).CombinedOutput()

	if !strings.Contains(string(out), agentName) {
		return fmt.Errorf("agent not found in vCluster %s view", tenant)
	}

	return nil
}

func RunCleanup(tenant, agentName string) error {
	fmt.Printf("🧹 Cleaning up test agent: %s...\n", agentName)
	k3s := "/usr/local/bin/k3s"
	vcluster := "/usr/local/bin/vcluster"

	// Delete from vCluster
	cleanupCmd := fmt.Sprintf("KUBECONFIG=/etc/rancher/k3s/k3s.yaml %s connect %s --namespace vcluster-%s -- %s kubectl delete pod %s --now",
		vcluster, tenant, tenant, k3s, agentName)

	return exec.Command("sh", "-c", cleanupCmd).Run()
}
