package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
)

type AgentRequest struct {
	ImageName string `json:"image" binding:"required"`
	AgentName string `json:"name" binding:"required"`
	Tenant    string `json:"tenant"`
}

func StartServer(port string) error {
	r := gin.Default()

	// 1. Hardware Status (Preserved nvidia-smi logic)
	r.GET("/status", func(c *gin.Context) {
		out, err := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu,utilization.memory,memory.used", "--format=csv,noheader,nounits").Output()
		gpuStatus := "Active"
		if err != nil {
			gpuStatus = "No GPU/Driver Missing"
		}

		c.JSON(http.StatusOK, gin.H{
			"gpu_status": gpuStatus,
			"metrics":    strings.TrimSpace(string(out)),
		})
	})

	// 3. Security Audit (Preserved log streaming)
	r.POST("/v1/agent/deploy", func(c *gin.Context) {
		var req AgentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid manifest"})
			return
		}

		targetTenant := req.Tenant
		if targetTenant == "" {
			targetTenant = "agent-enclave"
		}

		fmt.Printf("🚀 Deploying Agent: %s (%s) into Tenant: %s\n", req.AgentName, req.ImageName, targetTenant)

		// Using absolute paths
		vclusterBin := "/usr/local/bin/vcluster"
		k3sBin := "/usr/local/bin/k3s"
		kubeConfigPath := os.Getenv("HOME") + "/.kube/config"

		// Removed --kube-config flag to fix deprecation warning
		args := []string{
			"connect", targetTenant,
			"--namespace", "vcluster-" + targetTenant,
			"--", k3sBin, "kubectl", "run", req.AgentName, "--image=" + req.ImageName,
		}

		cmd := exec.Command(vclusterBin, args...)

		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeConfigPath)

		// CRITICAL: Explicitly set KUBECONFIG in the environment for this specific execution
		// This prevents vcluster from looking at 127.0.0.1:6443 via a default config
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeConfigPath)

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ vCluster Err: %v | Output: %s\n", err, string(output))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "vCluster deployment failed",
				"details": string(output),
			})
			return
		}

		injectAuditPolicy(req.AgentName)
		c.JSON(http.StatusAccepted, gin.H{"status": "Enclave secured", "agent": req.AgentName, "tenant": targetTenant})
	})

	r.GET("/v1/agent/audit", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")

		cmd := exec.Command("/usr/local/bin/k3s", "kubectl", "logs", "-n", "kube-system", "-l", "app.kubernetes.io/name=tetragon", "-c", "tetragon", "--follow")
		stdout, _ := cmd.StdoutPipe()
		cmd.Start()

		defer cmd.Process.Kill()
		io.Copy(c.Writer, stdout)
	})

	return r.Run(":" + port)
}

func injectAuditPolicy(agentName string) {
	policy := fmt.Sprintf(`
apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: "audit-%s"
spec:
  kprobes:
  - call: "security_file_open"
    syscall: false
    args:
    - index: 0
      type: "file"
    selectors:
    - matchActions:
      - action: Post
`, agentName)

	cmd := exec.Command("sh", "-c", "KUBECONFIG=/etc/rancher/k3s/k3s.yaml /usr/local/bin/k3s kubectl apply -f -")
	cmd.Stdin = strings.NewReader(policy)
	cmd.Run()
}
