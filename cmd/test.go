package cmd

import (
	"fmt"
	"os"

	"vecta/internal/test" // Ensure this matches your go.mod module name
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run the Vecta integration and security test suite",
	Long:  `Executes a series of automated sanity checks, deployment tests, and landing verifications.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Define the target for the test
		tenant := "agent-enclave"
		agentName := "sanity-check-agent"

		fmt.Println("🧪 Vecta Test Suite: Initializing...")
		fmt.Printf("📍 Targeting Tenant: %s\n\n", tenant)

		// Extensible Test Case Slice
		tests := []test.TestCase{
			{
				Name: "API: Sanity & GPU Metrics Check",
				Run:  test.RunSanity,
			},
			{
				Name: "Deployment: Multi-Tenant Landing Check",
				Run: func() error {
					return test.RunLandingTest(tenant, agentName)
				},
			},
			{
    				Name: "Cleanup: Remove Test Resources",
    				Run: func() error {
        				return test.RunCleanup(tenant, agentName)
    				},
			},
		}

		// Execution Loop
		passed := 0
		for _, t := range tests {
			fmt.Printf("--- %-40s ", t.Name)
			if err := t.Run(); err != nil {
				fmt.Printf("❌ FAILED\n    👉 Error: %v\n", err)
			} else {
				fmt.Println("✅ PASSED")
				passed++
			}
		}

		// Summary
		fmt.Printf("\n🏁 Test Summary: %d/%d passed\n", passed, len(tests))
		if passed < len(tests) {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}

