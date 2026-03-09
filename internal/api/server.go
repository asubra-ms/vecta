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

// Updated to include AuditDuration and Tenant from CLI
type AgentRequest struct {
	ImageName     string `json:"image" binding:"required"`
	AgentName     string `json:"name" binding:"required"`
	Tenant        string `json:"tenant"`
	AuditDuration string `json:"audit_duration"`
}

func StartServer(port string) error {
	r := gin.Default()

	// 1. Hardware Status (Preserved)
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

	// 2. Deploy Agent with Tenant-First Isolation logic
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
		fmt.Printf("🚀 Hardening & Deploying Agent: %s [Audit: %s] for Tenant: %s\n", req.AgentName, req.AuditDuration, targetTenant)

		// Update Volume Paths based on Tenant
		tenantRoot := fmt.Sprintf("/var/vecta/%s", targetTenant)
		agentRoot := fmt.Sprintf("%s/agents/%s", tenantRoot, req.AgentName)

		// Ensure these directories exist on the host before the pod starts
		_ = exec.Command("sudo", "mkdir", "-p", tenantRoot+"/policy", agentRoot+"/lib", agentRoot+"/logs").Run()
		_ = exec.Command("sudo", "chmod", "-R", "777", agentRoot).Run()
		// Keep the tenant's policy restricted (Standard 755 or 644))
		_ = exec.Command("sudo", "chmod", "755", tenantRoot+"/policy").Run()

		// A. Register Identity in SPIRE (Using vecta.io trust domain)
		err := identity.RegisterEnclaveIdentity(req.AgentName, namespace)
		if err != nil {
			fmt.Printf("❌ Identity Registry Err: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Identity registration failed"})
			return
		}

		// B. Construct Pod Manifest with Isolated Sovereign Volume Mounts
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{
				Name: req.AgentName,
				Labels: map[string]string{
					"vecta.io/tenant": targetTenant,
					"vecta.io/agent":  req.AgentName,
				},
				Annotations: map[string]string{
					"vecta.io/audit-duration": req.AuditDuration,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "agent",
						Image: req.ImageName,
						Env: []corev1.EnvVar{
							{Name: "VECTA_SENTRY_URL", Value: "http://localhost:8000"},
						},
					},
					{
						Name:  "sentry-warden",
						Image: "localhost:5000/vecta-sentry:latest",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "policy-vol",
								MountPath: "/var/vecta/policy",
								ReadOnly:  true,
							},
							{
								Name:      "lib-vol",
								MountPath: "/var/vecta/lib", // Maps to tenant/agent subpath on host
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "policy-vol",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: tenantRoot + "/policy", // Shared by agents in same tenant
							},
						},
					},
					{
						Name: "lib-vol",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: agentRoot + "/lib", // Exclusive to this agent instance
							},
						},
					},
				},
			},
		}

		// C. Apply Security Patch (Preserved)
		_, err = webhook.CreatePatch(pod, targetTenant)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate security patch"})
			return
		}

		// D. Execute Deployment via vCluster (Preserved with Environment fix)
		kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
		deployCmd := fmt.Sprintf("/usr/local/bin/vcluster connect %s --namespace %s -- /usr/local/bin/k3s kubectl apply -f -", targetTenant, namespace)

		cmd := exec.Command("sh", "-c", deployCmd)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeConfigPath)

		podJSON, _ := json.Marshal(pod)
		cmd.Stdin = strings.NewReader(string(podJSON))

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ vCluster Err: %v | Output: %s\n", err, string(output))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "vCluster deployment failed"})
			return
		}

		// E. Inject Tetragon Policy (Preserved)
		injectAuditPolicy(req.AgentName)

		c.JSON(http.StatusAccepted, gin.H{"status": "Enclave secured", "agent": req.AgentName})
	})

	// 3. Security Audit Stream (Preserved)
	r.GET("/v1/agent/audit", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		cmd := exec.Command("/usr/local/bin/k3s", "kubectl", "logs", "-n", "kube-system", "-l", "app.kubernetes.io/name=tetragon", "-c", "tetragon", "--follow")
		stdout, _ := cmd.StdoutPipe()
		cmd.Start()
		defer cmd.Process.Kill()
		io.Copy(c.Writer, stdout)
	})

	return r.Run(":" + port)
}

// injectAuditPolicy remains focused on Tetragon L4/L6 monitoring (Preserved)
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
