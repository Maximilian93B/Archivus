#!/bin/bash

echo "🧪 Running Archivus Tests with Docker"
echo "======================================"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker Desktop."
    exit 1
fi

echo "✅ Docker is running"

# Clean up any existing test containers
echo "🧹 Cleaning up existing test containers..."
docker-compose -f docker-compose.test.yml down --volumes --remove-orphans 2>/dev/null || true

# Build and run tests
echo "🚀 Starting test environment..."
docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit

# Clean up after tests
echo "🧹 Cleaning up test environment..."
docker-compose -f docker-compose.test.yml down --volumes --remove-orphans

echo "✅ Test execution complete!" 