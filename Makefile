# Vecta Unified Build System
BINARY_NAME=vecta
BUILD_DIR=bin

.PHONY: all build clean start-server reset

all: build

# 1. Build the unified binary (CLI + API)
build:
	@echo "🔨 Building Vecta Unified Binary..."
	@mkdir -p $(BUILD_DIR)
	go mod tidy
	go build -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "✅ Build complete: ./$(BUILD_DIR)/$(BINARY_NAME)"

# 2. Start the Management API Server
# This uses the command defined in cmd/start_server.go
start-server: build
	@echo "🚀 Launching Vecta Orchestrator API..."
	./$(BUILD_DIR)/$(BINARY_NAME) start-server --port 8000

# 3. Teardown and Cleanup (Nuke option)
# This uses the command defined in cmd/reset.go
reset: build
	@echo "🧨 Initializing Vecta Reset..."
	sudo ./$(BUILD_DIR)/$(BINARY_NAME) reset --force

# 4. Clean build artifacts
clean:
	@echo "🧹 Cleaning up build artifacts..."
	rm -rf $(BUILD_DIR)
	go clean

