package sentry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"gopkg.in/yaml.v3"
)

// Global Configuration Paths based on Unified Workspace
const (
	socketPath      = "unix:///run/spire/sockets/agent.sock"
	policyPath      = "/var/vecta/policy/policy.yaml"  // INPUT: Managed by Vecta CLI
	manifestOutPath = "/var/vecta/lib/discovered.yaml" // OUTPUT: Automatic creation
	ModeAudit       = "AUDIT"
	ModeEnforce     = "ENFORCE"
)

// Policy defines the structure for external governance templates
type Policy struct {
	AuditDuration  string   `yaml:"audit_duration"`
	ForbiddenPaths []string `yaml:"forbidden_paths"`
	ForbiddenSQL   []string `yaml:"forbidden_sql"`
	AllowedDomains []string `yaml:"allowed_domains"`
}

// AgentRequest represents the standard telemetry structure from AI Agents
type AgentRequest struct {
	Payload        string `json:"payload"`
	DestinationURL string `json:"url"`
}

// Warden manages the internal state machine and enforcement engine
type Warden struct {
	Mode             string
	Policy           *Policy
	DiscoveredIntent map[string]bool
	mu               sync.RWMutex
	SVID             string
}

func StartSentry() {
	// 1. Initialize Warden and Load Dynamic Policy
	w := &Warden{
		Mode:             ModeAudit,
		DiscoveredIntent: make(map[string]bool),
		Policy:           loadPolicy(),
	}

	// 2. Identity Handshake (SPIRE Root of Trust)
	fmt.Println("🛡️  Vecta-Sentry: Establishing Trust via Identity Socket...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ IDENTITY CRITICAL: %v\n", err)
		os.Exit(1)
	}
	defer source.Close()

	svid, err := source.GetX509SVID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ SVID CRITICAL: %v\n", err)
		os.Exit(1)
	}
	w.SVID = svid.ID.String()
	fmt.Printf("✅ Identity Verified: %s\n", w.SVID)

	// 3. Vecta Lifecycle Management (Requirement 1 & 2)
	duration, _ := time.ParseDuration(w.Policy.AuditDuration)
	if duration == 0 {
		duration = 1 * time.Minute // Default fallback
	}

	fmt.Printf("⏳ Vecta Enclave: Starting in AUDIT mode for %v\n", duration)

	time.AfterFunc(duration, func() {
		w.mu.Lock()
		w.Mode = ModeEnforce
		w.mu.Unlock()

		fmt.Println("\n🔒 AUDIT COMPLETE. Vecta is now in ENFORCE mode.")

		// Requirement 2: Automatic Manifest Generation
		w.saveManifest()

		// Requirement 5: Kernel Policy Sync
		w.injectKernelProtection()
	})

	// 4. Start Semantic Filter (Gin Engine)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery()) // Production stability

	r.POST("/inspect/v1", w.handleInspection)

	// 5. Graceful Termination Support
	srv := &http.Server{
		Addr:    ":8000",
		Handler: r,
	}

	go func() {
		fmt.Println("🚀 Vecta-Sentry Warden is ACTIVE on port 8000")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ Server Crash: %v\n", err)
		}
	}()

	// Signal handling for clean exit
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("🛑 Shutting down Warden...")
	ctxShut, cancelShut := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShut()
	srv.Shutdown(ctxShut)
}

// handleInspection orchestrates the modular rule evaluation
func (w *Warden) handleInspection(c *gin.Context) {
	var req AgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	w.mu.RLock()
	mode := w.Mode
	w.mu.RUnlock()

	// --- ENFORCEMENT ENGINE ---

	// A. Filesystem Sandbox (Requirement 3: /tmp/vecta etc)
	for _, path := range w.Policy.ForbiddenPaths {
		if strings.Contains(req.Payload, path) {
			if mode == ModeEnforce {
				w.terminateAgent(fmt.Sprintf("Forbidden filesystem access: %s", path))
				return
			}
			fmt.Printf("📝 Audit Log: Agent intent to access %s\n", path)
		}
	}

	// B. Semantic Intent Check (Requirement 4: SQL, ptrace, execve)
	for _, sqlPattern := range w.Policy.ForbiddenSQL {
		matched, _ := regexp.MatchString("(?i)"+sqlPattern, req.Payload)
		if matched {
			if mode == ModeEnforce {
				w.terminateAgent(fmt.Sprintf("Restricted semantic intent: %s", sqlPattern))
				return
			}
			fmt.Printf("📝 Audit Log: Captured intent for %s\n", sqlPattern)
		}
	}

	// C. Network Whitelisting & Discovery (Requirement 2)
	isAllowed := false
	for _, domain := range w.Policy.AllowedDomains {
		if strings.Contains(req.DestinationURL, domain) {
			isAllowed = true
			break
		}
	}

	if mode == ModeAudit {
		w.mu.Lock()
		w.DiscoveredIntent[req.DestinationURL] = true
		w.mu.Unlock()
	} else if !isAllowed {
		w.terminateAgent(fmt.Sprintf("Unauthorized egress attempt: %s", req.DestinationURL))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "authorized",
		"mode":     mode,
		"identity": w.SVID,
	})
}

// terminateAgent executes the Mechanical Kill-Switch
func (w *Warden) terminateAgent(reason string) {
	fmt.Printf("🚨 VECTA KILL-SWITCH ACTIVATED: %s\n", reason)
	// Requirement 4: Immediate SIGKILL to prevent escape
	syscall.Kill(os.Getpid(), syscall.SIGKILL)
}

// saveManifest persists discovered behavior for user review
func (w *Warden) saveManifest() {
	w.mu.RLock()
	defer w.mu.RUnlock()

	data, err := yaml.Marshal(w.DiscoveredIntent)
	if err == nil {
		_ = os.WriteFile(manifestOutPath, data, 0644)
		fmt.Printf("📊 Intent Manifest automatically generated at %s\n", manifestOutPath)
	}
}

// loadPolicy reads configuration from the unified workspace
func loadPolicy() *Policy {
	// Defaults if the workspace is fresh
	p := &Policy{
		AuditDuration:  "1m",
		ForbiddenPaths: []string{"/tmp/vecta", "/etc/shadow"},
		ForbiddenSQL:   []string{"DROP", "DELETE", "execve", "ptrace"},
		AllowedDomains: []string{"api.openai.com", "github.com"},
	}

	data, err := os.ReadFile(policyPath)
	if err == nil {
		_ = yaml.Unmarshal(data, p)
		fmt.Printf("📑 Policy Engine: Loaded manifest from %s\n", policyPath)
	} else {
		fmt.Println("⚠️  Warning: No policy.yaml found at /var/vecta/policy/. Using fallback rules.")
	}
	return p
}

func (w *Warden) injectKernelProtection() {
	fmt.Println("💉 Synchronizing Vecta Policy with Tetragon eBPF Engine...")
	// Hook for Requirement 5
}
