#!/bin/bash
# 1. Ensure Policy is in place
echo "Setting up Ironclad Policy..."
sudo mkdir -p /etc/vecta
sudo cp policy.yaml /etc/vecta/policy.yaml

# 2. Run the Warden (assuming it's built)
./bin/sentry-warden &
WARDEN_PID=$!

# 3. Wait for Audit Mode to expire (e.g., if set to 10s for testing)
echo "Waiting for transition to ENFORCE mode..."
sleep 15 

# 4. Simulate a Violation via the API
echo "Simulating /tmp/vecta deletion attempt..."
curl -X POST http://localhost:8000/inspect/v1 \
  -H "Content-Type: application/json" \
  -d '{"payload": "rm -rf /tmp/vecta", "url": "api.openai.com"}'

# 5. Check if Warden is still alive
if ps -p $WARDEN_PID > /dev/null; then
   echo "❌ Failure: Warden did not trigger Kill-Switch."
else
   echo "✅ Success: Warden issued SIGKILL and terminated."
fi

