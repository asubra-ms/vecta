# Vecta Unified Build System
SHELL := /bin/bash
BINARY_NAME=vecta
SENTRY_NAME=sentry-warden
BUILD_DIR=bin
REGISTRY=localhost:5000
SENTRY_IMAGE=$(REGISTRY)/vecta-sentry:latest
# CHANGED: Updated Vecta Home to sovereign standard
VECTA_ROOT=/usr/local/vecta

# SPIRE Sovereign Identity Assets
SPIRE_DIR=infra/spire-server
SPIRE_IMAGE=vecta/spire-server:clean

# Tests and test agents
TEST_AGENT_IMAGE=$(REGISTRY)/agent-fs:latest

.PHONY: all build build-sentry clean start-server reset push-sentry workspace install-policy spire-assets build-test-agent test


# Ensure the local registry is running
ensure-registry:
	@if ! docker ps | grep -q vecta-registry; then \
		echo "🚀 Starting Local Sovereign Registry..."; \
		docker run -d -p 5000:5000 --restart=always --name vecta-registry registry:2 || true; \
	fi

# Update build-test-agent to depend on the registry
build-test-agent: ensure-registry
	@echo "🐍 Building Filesystem Test Agent..."
	sudo docker build --network=host -t $(TEST_AGENT_IMAGE) ./tests/chaos-agents/agent-fs
	sudo docker push $(TEST_AGENT_IMAGE)


# The Master Test Target
test: all build-test-agent push-sentry
	@echo "🧪 Running Automated Filesystem Enforcement Test..."
	@pgrep vecta > /dev/null || (echo "❌ Error: 'vecta start-server' must be running!" && exit 1)
	
	@echo "1. Deploying agent-fs with 30s Audit window..."
	@./$(BUILD_DIR)/$(BINARY_NAME) deploy --image $(TEST_AGENT_IMAGE) --name fs-tester --audit-time 45s || (echo "❌ Deployment Failed!" && exit 1)
	
	@echo "2. Waiting for transition and violation (45s)..."
	@sleep 65
	
	@echo "3. Verifying Kill-Switch Result..."
	@vcluster connect agent-enclave --namespace vcluster-agent-enclave -- \
		kubectl get pod fs-tester -o jsonpath='{.status.containerStatuses[0].state.terminated.exitCode}' | grep -q "137" && echo "✅ TEST PASSED" || (echo "❌ TEST FAILED"; exit 1)
	@if ./$(BUILD_DIR)/$(BINARY_NAME) status | grep -q "fs-tester"; then \
		echo "❌ TEST FAILED: Agent survived forbidden access."; \
		exit 1; \
	else \
		echo "✅ TEST PASSED: Agent terminated via SIGKILL."; \
	fi

all: workspace build build-sentry

# 1. Initialize the Sovereign Workspace (Production Setup)
workspace:
	@echo "🏗️  Initializing Vecta Workspace at $(VECTA_ROOT)..."
	# CHANGED: Added bin and lib subdirectories
	@sudo mkdir -p $(VECTA_ROOT)/policy $(VECTA_ROOT)/lib $(VECTA_ROOT)/logs $(VECTA_ROOT)/bin $(VECTA_ROOT)/version
	@sudo chown -R $(USER):$(USER) $(VECTA_ROOT)
	@sudo chmod -R 755 $(VECTA_ROOT)
	@echo "V3.0.0" | sudo tee $(VECTA_ROOT)/version/current > /dev/null

# 2. Build the unified binary (CLI + API) - Depends on spire-assets
build: spire-assets
	@echo "🪪  Preparing Sovereign SPIRE Identity Assets..."
	@mkdir -p $(BUILD_DIR)/$(SPIRE_DIR)
	@cp $(SPIRE_DIR)/configmap.yaml $(BUILD_DIR)/$(SPIRE_DIR)/ 2>/dev/null || true
	@cp $(SPIRE_DIR)/spire-server-sovereign.yaml $(BUILD_DIR)/$(SPIRE_DIR)/ 2>/dev/null || true
	sudo docker build -t $(SPIRE_IMAGE) ./$(SPIRE_DIR)
	sudo docker save $(SPIRE_IMAGE) | sudo /usr/local/bin/k3s ctr -n k8s.io images import -

# 3. Build the Sentry Warden and the Docker Image
build-sentry:
	@echo "🛡️  Building Vecta Sentry (Warden) Binary..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(SENTRY_NAME) ./internal/sentry
	sudo chmod +x $(BUILD_DIR)/$(SENTRY_NAME)
	@echo "🐳 Building Docker Image: $(SENTRY_IMAGE)"
	sudo docker build -t $(SENTRY_IMAGE) -f Dockerfile.sentry .
	@echo "✅ Sentry image built and tagged."

# 4. Install Default Policy (For Testing/Init)
install-policy:
	@echo "📑 Sealing default policy to $(VECTA_ROOT)/policy/policy.yaml..."
	@sudo cp policy.yaml $(VECTA_ROOT)/policy/policy.yaml 2>/dev/null || \
	 echo -e "audit_duration: \"1m\"\nforbidden_paths:\n  - \"/tmp/vecta\"\nforbidden_sql:\n  - \"drop\"\n  - \"execve\"" | \
	 sudo tee $(VECTA_ROOT)/policy/policy.yaml > /dev/null
	@sudo chown root:root $(VECTA_ROOT)/policy/policy.yaml
	@sudo chmod 644 $(VECTA_ROOT)/policy/policy.yaml

# 5. Push Sentry to Local Registry
push-sentry:
	@echo "📦 Pushing Sentry image to local registry..."
	sudo docker push $(SENTRY_IMAGE)

# 6. Start the Management API Server
start-server: build
	@echo "🚀 Launching Vecta Orchestrator API..."
	./$(BUILD_DIR)/$(BINARY_NAME) start-server --port 8000


# 7. Teardown and Cleanup
reset: build
	@echo "🧨 Initializing Vecta Reset..."
	sudo ./$(BUILD_DIR)/$(BINARY_NAME) reset --force
	@echo "🧹 Purging Vecta Workspace..."
	# CHANGED: Purging the new root
	sudo rm -rf $(VECTA_ROOT)

clean:
	@echo "🧹 Cleaning up build artifacts..."
	rm -rf $(BUILD_DIR)
	go clean

