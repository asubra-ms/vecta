package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Println("🤖 Vecta Test Agent: Online")
	
	for {
		// 1. Legitimate action: Simulate checking a local LLM endpoint
		fmt.Println("📡 [LOG] Pinging local-llm-service...")
		
		// 2. Sensitive action: The "Bait" to test Tetragon monitoring
		fmt.Println("🕵️  [AUDIT] Attempting to read sensitive host file: /etc/shadow")
		data, err := os.ReadFile("/etc/shadow")
		if err != nil {
			fmt.Printf("🔒 [RESULT] Access Denied by OS: %v\n", err)
		} else {
			fmt.Printf("🔓 [RESULT] Breach successful! Read %d bytes\n", len(data))
		}

		time.Sleep(10 * time.Second)
	}
}

