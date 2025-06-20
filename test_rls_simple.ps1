# Simple RLS Test
Write-Host "TESTING RLS POLICIES" -ForegroundColor Yellow
Write-Host "===================="

# Test login first
Write-Host "1. Testing Login..." -ForegroundColor Cyan
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$loginData = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $loginResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $headers -Body $loginData
    Write-Host "✅ Login successful!" -ForegroundColor Green
    $token = $loginResponse.token
    Write-Host "   Token: $($token.Substring(0,50))..." -ForegroundColor Gray
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test documents endpoint
Write-Host "`n2. Testing Documents Endpoint..." -ForegroundColor Cyan
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

try {
    $documentsResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/documents" -Method GET -Headers $headers
    Write-Host "✅ Documents endpoint accessible!" -ForegroundColor Green
    Write-Host "   Found $($documentsResponse.data.Count) documents" -ForegroundColor Gray
} catch {
    Write-Host "❌ Documents endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   Status Code: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
}

Write-Host "`nTest Complete" -ForegroundColor Yellow 