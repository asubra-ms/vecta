#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "🚀 Starting SPIRE Sovereign Bootstrap..."

# 1. Build and push to the local registry
# We tag it specifically as localhost:5000 so the YAML can pull it
docker build -t localhost:5000/spire-server:clean $DIR
docker push localhost:5000/spire-server:clean

# 2. Apply Config and Manifest
kubectl apply -f $DIR/configmap.yaml
kubectl apply -f $DIR/spire-server-sovereign.yaml

# 3. The Nuclear Reset
kubectl delete pod spire-server-0 -n spire --force --grace-period=0 2>/dev/null || true

echo "✅ SPIRE initialized. Waiting for readiness..."
kubectl wait --for=condition=ready pod/spire-server-0 -n spire --timeout=60s

