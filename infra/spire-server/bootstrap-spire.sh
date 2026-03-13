#!/bin/bash
#
set -e

# Always resolve relative to the script location
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "🚀 Starting SPIRE Sovereign Bootstrap..."

# 1. Build and side-load instead of just pushing
# This matches the Makefile's logic to guarantee k3s availability
docker build -t localhost:5000/spire-server:clean "$DIR"
docker save localhost:5000/spire-server:clean | sudo /usr/local/bin/k3s ctr -n k8s.io images import -

# 2. Apply Config and Manifest from the source directory
kubectl apply -f "$DIR/configmap.yaml"
kubectl apply -f "$DIR/spire-server-sovereign.yaml"

# 3. Reset the server to ensure it picks up the imported image
kubectl delete pod spire-server-0 -n spire --force --grace-period=0 2>/dev/null || true

echo "✅ SPIRE initialized via side-load. Waiting for readiness..."
kubectl wait --for=condition=ready pod/spire-server-0 -n spire --timeout=60s

