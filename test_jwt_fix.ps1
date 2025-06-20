# Test JWT Token Validation After Environment Setup
Write-Host "🔐 JWT TOKEN VALIDATION TEST" -ForegroundColor Cyan
Write-Host "============================" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080/api/v1"
$existingTenant = "testdebug123"
$adminEmail = "admin@testdebug123.com"
$adminPassword = "SecurePass123!"

# Step 1: Health check
Write-Host "`n1. Testing server health..." -ForegroundColor Yellow
try {
    $healthResponse = Invoke-RestMethod -Uri "http://localhost:8080/health"
    Write-Host "✅ Server is healthy" -ForegroundColor Green
} catch {
    Write-Host "❌ Server health check failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Step 2: Login
Write-Host "`n2. Testing login..." -ForegroundColor Yellow
$loginBody = @{
    email = $adminEmail
    password = $adminPassword
} | ConvertTo-Json

$loginHeaders = @{
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = $existingTenant
}

try {
    $loginResponse = Invoke-RestMethod -Uri "$baseUrl/auth/login" -Method POST -Body $loginBody -Headers $loginHeaders
    Write-Host "✅ Login successful!" -ForegroundColor Green
    $authToken = $loginResponse.token
    $tenantId = $loginResponse.user.tenant_id
    Write-Host "   Token: $($authToken.Substring(0,50))..." -ForegroundColor Gray
    Write-Host "   Tenant ID: $tenantId" -ForegroundColor Gray
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Step 3: Test protected endpoint (documents list)
Write-Host "`n3. Testing JWT token validation..." -ForegroundColor Yellow
$authHeaders = @{
    "Authorization" = "Bearer $authToken"
    "X-Tenant-ID" = $tenantId
}

try {
    $listResponse = Invoke-RestMethod -Uri "$baseUrl/documents" -Headers $authHeaders
    Write-Host "✅ JWT token validation works!" -ForegroundColor Green
    Write-Host "   Documents endpoint accessible" -ForegroundColor Gray
    Write-Host "   Response: $($listResponse | ConvertTo-Json -Depth 1)" -ForegroundColor Gray
} catch {
    Write-Host "❌ JWT token validation failed: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   This indicates the JWT_SECRET is still not configured correctly" -ForegroundColor Yellow
    exit 1
}

Write-Host "`n🎉 JWT TOKEN VALIDATION SUCCESS!" -ForegroundColor Green
Write-Host "==============================='" -ForegroundColor Green
Write-Host "✅ Server health" -ForegroundColor Green
Write-Host "✅ User authentication" -ForegroundColor Green  
Write-Host "✅ JWT token validation" -ForegroundColor Green
Write-Host "✅ Protected endpoints accessible" -ForegroundColor Green
Write-Host ""
Write-Host "🚀 Ready to run full Phase 2 tests!" -ForegroundColor Cyan 