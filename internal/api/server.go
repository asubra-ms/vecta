package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"vecta/internal/identity"
	"vecta/internal/webhook"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// 2. Deploy Agent with Hardened Sidecar Injection
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

		namespace := "vcluster-" + targetTenant
		fmt.Printf("🚀 Hardening & Deploying Agent: %s into Tenant: %s\n", req.AgentName, targetTenant)

		// --- VECTA HARDENING LOGIC START ---

		// A. Register Identity in SPIRE
		err := identity.RegisterEnclaveIdentity(req.AgentName, namespace)
		if err != nil {
			fmt.Printf("❌ Identity Registry Err: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Identity registration failed"})
			return
		}

		// B. Construct Pod Manifest for Patching
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{
				Name: req.AgentName,
				Labels: map[string]string{
					"vecta.io/tenant": targetTenant,
					"vecta.io/agent":  req.AgentName,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "agent",
						Image: req.ImageName,
					},
				},
			},
		}

		// C. Generate JSON Patch (Injects Sentry Sidecar & SPIRE CSI)
		// We use the patch logic but apply it directly to the object here
		// for clean deployment via Stdin.
		_, err = webhook.CreatePatch(pod, targetTenant) // This generates the sidecar & volumes
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate security patch"})
			return
		}

		// --- VECTA HARDENING LOGIC END ---

		vclusterBin := "/usr/local/bin/vcluster"
		k3sBin := "/usr/local/bin/k3s"
		kubeConfigPath := os.Getenv("HOME") + "/.kube/config"

		// Using 'kubectl apply' with the hardened manifest via Stdin
		deployCmd := fmt.Sprintf("%s connect %s --namespace %s -- %s kubectl apply -f -",
			vclusterBin, targetTenant, namespace, k3sBin)

		cmd := exec.Command("sh", "-c", deployCmd)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeConfigPath)

		// Hardened Pod is marshaled and piped to the enclave
		podJSON, _ := json.Marshal(pod)
		cmd.Stdin = strings.NewReader(string(podJSON))

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ vCluster Err: %v | Output: %s\n", err, string(output))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "vCluster deployment failed", "details": string(output)})
			return
		}

		injectAuditPolicy(req.AgentName)
		c.JSON(http.StatusAccepted, gin.H{"status": "Enclave secured", "agent": req.AgentName, "tenant": targetTenant})
	})

	// 3. Security Audit (Preserved log streaming)
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
