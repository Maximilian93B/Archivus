# Test admin login for freshtest tenant
Write-Host "Testing admin login for 'freshtest' tenant..." -ForegroundColor Cyan

$loginPayload = @{
    email = "admin@freshtest.com"
    password = "SecurePass123!"
} | ConvertTo-Json

$loginHeaders = @{
    "X-Tenant-Subdomain" = "freshtest"
    "Content-Type" = "application/json"
}

try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $loginHeaders -Body $loginPayload
    Write-Host "SUCCESS: Admin login worked!" -ForegroundColor Green
    Write-Host "User: $($response.user.email) ($($response.user.role))"
    Write-Host "Token: $($response.token.Substring(0,50))..."
} catch {
    Write-Host "ERROR: Admin login failed" -ForegroundColor Red
    Write-Host "Status: $($_.Exception.Response.StatusCode)"
    Write-Host "Details: $($_.Exception.Response.StatusDescription)"
} 