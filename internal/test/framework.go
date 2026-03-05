package test

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

type TestAgent struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	Tenant string `json:"tenant"`
}

// SanityCheck verifies the API is alive
func SanityCheck(apiUrl string) error {
	resp, err := http.Get(apiUrl + "/status")
	if err != nil || resp.StatusCode != 200 {
		return fmt.Errorf("API Status Check Failed: %v", err)
	}
	return nil
}

// VerifyPlacement checks if the pod exists in both Host and Virtual contexts
func VerifyPlacement(tenant, podName string) error {
	k3s := "/usr/local/bin/k3s"
	vcluster := "/usr/local/bin/vcluster"
	
	// Check Host Namespace
	hostCmd := exec.Command(k3s, "kubectl", "get", "pods", "-n", "vcluster-"+tenant)
	out, _ := hostCmd.CombinedOutput()
	if !strings.Contains(string(out), podName) {
		return fmt.Errorf("pod not found in host namespace vcluster-%s", tenant)
	}

	// Check Virtual Context
	virtCmd := exec.Command(vcluster, "connect", tenant, "--namespace", "vcluster-"+tenant, "--", k3s, "kubectl", "get", "pods")
	out, _ = virtCmd.CombinedOutput()
	if !strings.Contains(string(out), podName) {
		return fmt.Errorf("pod not found inside virtual cluster %s", tenant)
	}

	return nil
}

