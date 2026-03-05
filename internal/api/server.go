package api

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
)

type AgentRequest struct {
	ImageName string `json:"image" binding:"required"`
	AgentName string `json:"name" binding:"required"`
	Tenant    string `json:"tenant"` // Added: Option for named vClusters
}

// StartServer launches the Vecta Orchestrator API
func StartServer(port string) error { // Added port parameter for CLI flexibility
	r := gin.Default()

	// 1. Hardware Status: PRESERVED nvidia-smi logic
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

	// 2. Customer Endpoint: UPDATED with Tenant awareness
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

    // Use absolute paths for everything to avoid PATH issues in sudo
    k3s := "/usr/local/bin/k3s"
	vcluster := "/usr/local/bin/vcluster"

    // Construct the command as a slice to avoid shell injection and path issues
    args := []string{
        "connect", targetTenant,
        "--namespace", "vcluster-" + targetTenant,
        "--", k3s, "kubectl", "run", req.AgentName, "--image=" + req.ImageName,
    }

    cmd := exec.Command(vcluster, args...)

    // CRITICAL: Inject the KUBECONFIG so the command knows which cluster to talk to
    cmd.Env = append(os.Environ(), "KUBECONFIG=/etc/rancher/k3s/k3s.yaml")

    output, err := cmd.CombinedOutput()
    if err != nil {
        fmt.Printf("❌ vCluster Err: %v | Output: %s\n", err, string(output))
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "vCluster deployment failed",
            "details": string(output), // This will show us the real error in the test logs
        })
        return
    }

    injectAuditPolicy(req.AgentName)
    c.JSON(http.StatusAccepted, gin.H{"status": "Enclave secured", "agent": req.AgentName})
})




	// 3. Security Audit: PRESERVED log streaming logic
	r.GET("/v1/agent/audit", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")

		// Using absolute path for kubectl via k3s
		cmd := exec.Command("/usr/local/bin/k3s", "kubectl", "logs", "-n", "kube-system", "-l", "app.kubernetes.io/name=tetragon", "-c", "tetragon", "--follow")
		stdout, _ := cmd.StdoutPipe()
		cmd.Start()

		defer cmd.Process.Kill()

		io.Copy(c.Writer, stdout)
	})

	return r.Run(":" + port)

}

// injectAuditPolicy: PRESERVED your exact TracingPolicy
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

	// Using absolute path for K3s
	cmd := exec.Command("sh", "-c", "KUBECONFIG=/etc/rancher/k3s/k3s.yaml /usr/local/bin/k3s kubectl apply -f -")
	cmd.Stdin = strings.NewReader(policy)
	cmd.Run()
}

