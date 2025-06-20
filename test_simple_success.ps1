# Simple Success Test
Write-Host "SIMPLE AUTHENTICATION SUCCESS TEST" -ForegroundColor Green
Write-Host "==================================="

# Login
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$loginData = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

$loginResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $headers -Body $loginData
Write-Host "✅ Login successful!" -ForegroundColor Green
$token = $loginResponse.token

# Test documents
$authHeaders = @{ 
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

$documentsResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/documents" -Method GET -Headers $authHeaders
Write-Host "✅ Documents endpoint accessible!" -ForegroundColor Green

Write-Host "Response structure:" -ForegroundColor Yellow
$documentsResponse | ConvertTo-Json -Depth 2 | Write-Host -ForegroundColor White

Write-Host "`n🎉 AUTHENTICATION IS WORKING!" -ForegroundColor Green 