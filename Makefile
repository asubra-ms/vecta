# Vecta Unified Build System
BINARY_NAME=vecta
SENTRY_NAME=sentry-warden
BUILD_DIR=bin
REGISTRY=localhost:5000
SENTRY_IMAGE=$(REGISTRY)/vecta-sentry:latest

.PHONY: all build build-sentry clean start-server reset push-sentry

all: build build-sentry

# 1. Build the unified binary (CLI + API)
build:
	@echo "🔨 Building Vecta Unified Binary..."
	@mkdir -p $(BUILD_DIR)
	go mod tidy
	# Build the current directory (.) instead of main.go to resolve internal packages
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "✅ Build complete: ./$(BUILD_DIR)/$(BINARY_NAME)"

# 2. Build the Sentry Warden and the Docker Image
build-sentry:
	@echo "🛡️  Building Vecta Sentry (Warden) Binary..."
	@mkdir -p $(BUILD_DIR)
	# Cross-compile for the Alpine container
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(SENTRY_NAME) internal/sentry/main.go
	@echo "🐳 Building Docker Image: $(SENTRY_IMAGE)"
	sudo docker build -t $(SENTRY_IMAGE) -f Dockerfile.sentry .
	@echo "✅ Sentry image built and tagged."

# 3. Push Sentry to Local Registry
push-sentry:
	@echo "📦 Pushing Sentry image to local registry..."
	sudo docker push $(SENTRY_IMAGE)

# 4. Start the Management API Server
start-server: build
	@echo "🚀 Launching Vecta Orchestrator API..."
	./$(BUILD_DIR)/$(BINARY_NAME) start-server --port 8000

# 5. Teardown and Cleanup
reset: build
	@echo "🧨 Initializing Vecta Reset..."
	sudo ./$(BUILD_DIR)/$(BINARY_NAME) reset --force

# 6. Clean build artifacts
clean:
	@echo "🧹 Cleaning up build artifacts..."
	rm -rf $(BUILD_DIR)
	go clean

