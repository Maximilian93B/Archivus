@echo off
echo 🧪 Running Archivus Tests with Docker
echo ======================================

REM Check if Docker is running
docker info >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ Docker is not running. Please start Docker Desktop.
    exit /b 1
)

echo ✅ Docker is running

REM Clean up any existing test containers
echo 🧹 Cleaning up existing test containers...
docker-compose -f docker-compose.test.yml down --volumes --remove-orphans >nul 2>&1

REM Build and run tests
echo 🚀 Starting test environment...
docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit

REM Clean up after tests
echo 🧹 Cleaning up test environment...
docker-compose -f docker-compose.test.yml down --volumes --remove-orphans

echo ✅ Test execution complete!
pause 