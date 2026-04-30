#!/usr/bin/env bash
# gomemkv — build Docker image, start container, run integration tests
set -e

PROJECT=/Users/ericnguyen/DATA/Workspace/Backend/go/gomemkv
CONTAINER=gomemkv-test
IMAGE=gomemkv:local

echo "==> [1/4] Killing existing processes on port 9090..."
pkill -f gomemkv 2>/dev/null || true
docker stop $CONTAINER 2>/dev/null || true
sleep 1

echo "==> [2/4] Building Docker image..."
docker build -t $IMAGE "$PROJECT"

echo "==> [3/4] Starting container..."
docker run -d --name $CONTAINER -p 9090:9090 --rm $IMAGE
echo "    Waiting for server to be ready..."
sleep 2
docker ps | grep $CONTAINER && echo "    ✓ Container running" || (echo "    ✗ Container failed to start"; exit 1)

echo "==> [4/4] Running integration tests..."
cd "$PROJECT/test/redis"
npm test

echo "==> Stopping container..."
docker stop $CONTAINER 2>/dev/null || true
echo "Done."
