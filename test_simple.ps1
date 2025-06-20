# Simple Fresh User Test
$headers = @{
    'Content-Type' = 'application/json'
    'X-Tenant-Subdomain' = 'testcorp'
}

$email = "fresh.$(Get-Date -Format 'HHmmss')@testcorp.com"
Write-Host "Testing with: $email"

# Register
$regBody = @{
    email = $email
    password = 'FreshPass123!'
    first_name = 'Fresh'
    last_name = 'User'
} | ConvertTo-Json

try {
    $regResponse = Invoke-WebRequest -Uri 'http://localhost:8080/api/v1/auth/register' -Method POST -Body $regBody -Headers $headers -UseBasicParsing
    Write-Host "REG SUCCESS: $($regResponse.StatusCode)"
    
    # Login immediately
    $loginBody = @{
        email = $email
        password = 'FreshPass123!'
    } | ConvertTo-Json
    
    $loginResponse = Invoke-WebRequest -Uri 'http://localhost:8080/api/v1/auth/login' -Method POST -Body $loginBody -Headers $headers -UseBasicParsing
    Write-Host "LOGIN SUCCESS: $($loginResponse.StatusCode)"
    
    Write-Host "AUTHENTICATION WORKING!" -ForegroundColor Green
    
} catch {
    Write-Host "FAILED: $($_.Exception.Message)" -ForegroundColor Red
} 