# =============================================================================
# Archivus Authentication System Test
# =============================================================================
# 
# This script tests the complete authentication flow using pre-verified users
# created via the admin API to bypass Supabase email verification requirements.
#
# Key Insights from Development:
# - Supabase requires email verification for standard authentication
# - Admin API can create pre-verified users with EmailConfirm=true  
# - Our authentication logic is solid; the issue was email verification
# - Solution: Use admin endpoint for testing/development users
#
# Test Flow:
# 1. Health check - verify server is running
# 2. Create pre-verified user via admin API
# 3. Login with pre-verified user
# 4. Access protected endpoint with JWT token
#
# Usage: powershell -ExecutionPolicy Bypass -File test_authentication.ps1
# =============================================================================

# Simple authentication flow test
$baseUrl = "http://localhost:8080"
$headers = @{
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testcorp"
}

$testUser = @{
    email = "simple@testcorp.local"
    password = "SimpleP@ss123"
    first_name = "Simple"
    last_name = "TestUser"
    role = "admin"
    department = "IT"
    job_title = "System Administrator"
}

Write-Host "Testing Archivus Authentication Flow" -ForegroundColor Cyan

# Step 1: Health Check
Write-Host "1. Health check..." -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "$baseUrl/health" -Method Get
    Write-Host "   Health: PASSED" -ForegroundColor Green
} catch {
    Write-Host "   Health: FAILED" -ForegroundColor Red
    exit 1
}

# Step 2: Create user
Write-Host "2. Creating pre-verified user..." -ForegroundColor Yellow
$userBody = $testUser | ConvertTo-Json
try {
    $createResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/admin/create-verified-user" -Method Post -Body $userBody -Headers $headers
    Write-Host "   User Creation: PASSED" -ForegroundColor Green
    Write-Host "   Email Verified: $($createResponse.email_verified)" -ForegroundColor Gray
} catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 409) {
        Write-Host "   User Creation: SKIPPED (already exists)" -ForegroundColor Yellow
    } else {
        Write-Host "   User Creation: FAILED" -ForegroundColor Red
        Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
        exit 1
    }
}

# Step 3: Login
Write-Host "3. Testing login..." -ForegroundColor Yellow
$loginBody = @{
    email = $testUser.email
    password = $testUser.password
} | ConvertTo-Json

try {
    $loginResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method Post -Body $loginBody -Headers $headers
    Write-Host "   Login: PASSED" -ForegroundColor Green
    Write-Host "   User: $($loginResponse.user.first_name) $($loginResponse.user.last_name)" -ForegroundColor Gray
    $accessToken = $loginResponse.token
} catch {
    Write-Host "   Login: FAILED" -ForegroundColor Red
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Step 4: Protected endpoint
Write-Host "4. Testing protected endpoint..." -ForegroundColor Yellow
$authHeaders = $headers.Clone()
$authHeaders["Authorization"] = "Bearer $accessToken"

try {
    $profileResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/users/profile" -Method Get -Headers $authHeaders
    Write-Host "   Protected Endpoint: PASSED" -ForegroundColor Green
    Write-Host "   Profile: $($profileResponse.first_name) $($profileResponse.last_name)" -ForegroundColor Gray
} catch {
    Write-Host "   Protected Endpoint: FAILED" -ForegroundColor Red
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host "`nALL TESTS PASSED! Authentication system is working!" -ForegroundColor Green 