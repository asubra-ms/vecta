package sentry

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

type AgentRequest struct {
	Payload        string `json:"payload"`
	DestinationURL string `json:"url"`
}

func StartSentry() {
	r := gin.Default()

	// The Warden's Semantic Filter
	r.POST("/inspect/v1", func(c *gin.Context) {
		var req AgentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		// 1. SQL Injection / Intent Check (Regex)
		sqlPattern := `(?i)(DROP|DELETE|TRUNCATE)\s+TABLE`
		matched, _ := regexp.MatchString(sqlPattern, req.Payload)
		if matched {
			c.JSON(http.StatusForbidden, gin.H{"detail": "Vecta-Sentry: Malicious SQL Intent Blocked"})
			return
		}

		// 2. URL Whitelist Check
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

		c.JSON(http.StatusOK, gin.H{"status": "authorized"})
	})

	r.Run(":8000")
}
