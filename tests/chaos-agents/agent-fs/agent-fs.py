import os
import time
import requests
import sys

# Constants injected by the Vecta Sentry Warden
SENTRY_URL = os.getenv("VECTA_SENTRY_URL", "http://localhost:8000")
FORBIDDEN_PATH = "/tmp/vecta/violation.txt"

def report_intent(payload):
    """Signals intent to the Warden. In ENFORCE mode, this initiates the kill."""
    try:
        requests.post(f"{SENTRY_URL}/inspect/v1", 
                      json={"payload": payload, "url": "enclave-test.internal"}, 
                      timeout=2)
    except Exception as e:
        print(f"📡 Warden connection failed: {e}")

def main():
    print("🤖 Agent-FS: Starting sovereign filesystem test...")
    
    # 1. Audit Window 
    # We perform safe actions to ensure discovery is working.
    print("⏱️  Phase 1: Recording baseline intent (Audit Mode)...")
    report_intent("ls /home/agent")
    time.sleep(35) # We wait past the 30s audit window defined in 'make test'

    # 2. The Controlled Violation
    # Attempting to write to a path explicitly forbidden in /var/vecta/policy/policy.yaml
    print(f"🚨 Phase 2: Attempting forbidden write to {FORBIDDEN_PATH}")
    report_intent(f"touch {FORBIDDEN_PATH}")
    
    try:
        # The Warden should intercept this and SIGKILL the process immediately
        with open(FORBIDDEN_PATH, "w") as f:
            f.write("Mechanical Kill-Switch Validation")
    except Exception as e:
        print(f"Action interrupted: {e}")
    
    # If the process reaches this line, the Warden failed to enforce.
    print("❌ FAIL: Agent-FS survived forbidden filesystem access.")
    sys.exit(1)

if __name__ == "__main__":
    main()

