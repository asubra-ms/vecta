package api

import (
	"encoding/json"
	"fmt"
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

const KubeConfigPath = "/etc/rancher/k3s/k3s.yaml"

type AgentRequest struct {
	ImageName     string `json:"image" binding:"required"`
	AgentName     string `json:"name" binding:"required"`
	Tenant        string `json:"tenant"`
	AuditDuration string `json:"audit_duration"`
}

func StartServer(port string) error {
	r := gin.Default()

	r.GET("/status", func(c *gin.Context) {
		out, _ := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu,utilization.memory,memory.used", "--format=csv,noheader,nounits").Output()
		c.JSON(http.StatusOK, gin.H{"gpu_status": "Active", "metrics": strings.TrimSpace(string(out))})
	})

	r.POST("/v1/agent/deploy", func(c *gin.Context) {
		var req AgentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		fmt.Printf("🚀 Deploying Sovereign Agent: %s [%s Audit] for Tenant: %s\n", req.AgentName, req.AuditDuration, req.Tenant)

		// 1. Identity Registration
		if err := identity.RegisterVclusterIdentity(req.AgentName, "vcluster-"+req.Tenant); err != nil {
			fmt.Printf("⚠️  Identity Note: %v\n", err)
		}

		// 2. Pod Construction
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{
				Name: req.AgentName,
				Labels: map[string]string{
					"vecta-agent": req.AgentName,
					"tenant":      req.Tenant,
					"audit":       req.AuditDuration,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "agent", Image: req.ImageName}},
			},
		}

		// 3. Apply Vecta Patch (Injects Warden & Volumes)
		webhook.PatchPodForSovereignty(pod)

		// 4. Secure Deployment via vCluster Tunnel
		cmd := exec.Command("vcluster", "connect", req.Tenant, "-n", "vcluster-"+req.Tenant, "--insecure", "--", "/usr/local/bin/k3s", "kubectl", "apply", "-f", "-")
		cmd.Env = append(os.Environ(), "KUBECONFIG="+KubeConfigPath)

		podJSON, _ := json.Marshal(pod)
		cmd.Stdin = strings.NewReader(string(podJSON))

		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("❌ Deployment Error: %s\n", string(out))
			c.JSON(500, gin.H{"error": "vCluster deploy failed", "details": string(out)})
			return
		}

		injectAuditPolicy(req.AgentName)
		c.JSON(http.StatusAccepted, gin.H{"status": "Enclave secured", "agent": req.AgentName})
	})

	return r.Run(":" + port)
}

func injectAuditPolicy(agentName string) {
	policy := fmt.Sprintf(`
apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: "audit-fs-%s"
spec:
  kprobes:
  - call: "sys_openat"
    syscall: true
    args:
    - index: 1
      type: "string"
    selectors:
    - matchPIDs:
      - operator: In
        followForks: true
        isNamespacePID: true
        values: [1]
`, agentName)

	cmd := exec.Command("/usr/local/bin/k3s", "kubectl", "apply", "-f", "-")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+KubeConfigPath)
	cmd.Stdin = strings.NewReader(policy)
	_ = cmd.Run()
}
