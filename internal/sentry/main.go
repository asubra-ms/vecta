package sentry

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

type AgentRequest struct {
	Payload        string `json:"payload"`
	DestinationURL string `json:"url"`
}

const socketPath = "unix:///run/spire/sockets/agent.sock"

func StartSentry() {
	// 1. Identity Handshake (The Root of Trust)
	fmt.Println("🛡️  Vecta-Sentry: Connecting to Identity socket...")

	// Create a context with a timeout for the handshake
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the SPIRE Agent Socket
	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)))
	if err != nil {
		fmt.Printf("❌ Identity Failure: Could not reach SPIRE socket: %v\n", err)
		fmt.Println("🚨 Vecta-Sentry cannot verify host trust. Exiting.")
		return
	}
	defer source.Close()

	// Verify we can actually get an identity (SVID)
	svid, err := source.GetX509SVID()
	if err != nil {
		fmt.Printf("❌ Identity Failure: No SVID assigned: %v\n", err)
		return
	}
	fmt.Printf("✅ Identity Verified: %s\n", svid.ID)

	// 2. Start the Semantic Filter (Warden Logic)
	r := gin.Default()

	r.POST("/inspect/v1", func(c *gin.Context) {
		var req AgentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		// --- Semantic Filter Logic ---

		// A. SQL Injection / Intent Check (Regex)
		sqlPattern := `(?i)(DROP|DELETE|TRUNCATE)\s+TABLE`
		matched, _ := regexp.MatchString(sqlPattern, req.Payload)
		if matched {
			c.JSON(http.StatusForbidden, gin.H{"detail": "Vecta-Sentry: Malicious SQL Intent Blocked"})
			return
		}

		// B. URL Whitelist Check
		allowedDomains := []string{"api.openai.com", "github.com"}
		validURL := false
		for _, domain := range allowedDomains {
			if strings.Contains(req.DestinationURL, domain) {
				validURL = true
				break
			}
		}
		if !validURL {
			c.JSON(http.StatusForbidden, gin.H{"detail": "Vecta-Sentry: Unauthorized URL"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "authorized",
			"identity": svid.ID.String(), // Include identity in metadata
		})
	})

	fmt.Println("🚀 Vecta-Sentry Warden is ACTIVE on port 8000")
	r.Run(":8000")
}
