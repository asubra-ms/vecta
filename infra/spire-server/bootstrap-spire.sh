#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "🚀 Starting SPIRE Sovereign Bootstrap..."

# 1. Build the clean image locally
docker build -t vecta/spire-server:clean $DIR
docker save vecta/spire-server:clean | sudo k3s ctr images import -

# 2. Apply Config and Manifest
kubectl apply -f $DIR/configmap.yaml
kubectl apply -f $DIR/spire-server-sovereign.yaml

# 3. The Nuclear Reset (Delete PVC and Pod to ensure fresh vecta.io start)
kubectl delete pod spire-server-0 -n spire --force --grace-period=0 2>/dev/null || true

echo "✅ SPIRE initialized. Waiting for readiness..."
kubectl wait --for=condition=ready pod/spire-server-0 -n spire --timeout=60s

