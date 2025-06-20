# Test login with the regular user that was successfully created
$headers = @{
    'Content-Type' = 'application/json'
    'X-Tenant-Subdomain' = 'testcorp'
}

$loginBody = @{
    email = 'user@testcorp.com'
    password = 'SecureUser123!'
} | ConvertTo-Json

Write-Host "Testing login for regular user..." -ForegroundColor Yellow
Write-Host "Body: $loginBody"

try {
    $response = Invoke-WebRequest -Uri 'http://localhost:8080/api/v1/auth/login' -Method POST -Body $loginBody -Headers $headers -UseBasicParsing
    Write-Host "LOGIN SUCCESS!" -ForegroundColor Green
    
    $loginData = $response.Content | ConvertFrom-Json
    Write-Host "User: $($loginData.user.email)" -ForegroundColor Cyan
    Write-Host "Role: $($loginData.user.role)" -ForegroundColor Cyan
    Write-Host "Token received: $($loginData.token.Substring(0,20))..." -ForegroundColor Cyan
    
    # Test an authenticated endpoint
    Write-Host "`nTesting authenticated endpoint..." -ForegroundColor Yellow
    $authHeaders = @{
        'Authorization' = "Bearer $($loginData.token)"
        'Content-Type' = 'application/json'
        'X-Tenant-Subdomain' = 'testcorp'
    }
    
    $profileResponse = Invoke-WebRequest -Uri 'http://localhost:8080/api/v1/users/profile' -Method GET -Headers $authHeaders -UseBasicParsing
    Write-Host "PROFILE SUCCESS!" -ForegroundColor Green
    $profileData = $profileResponse.Content | ConvertFrom-Json
    Write-Host "Profile: $($profileData.first_name) $($profileData.last_name)" -ForegroundColor Cyan
    
} catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
    
    if ($_.Exception.Response) {
        $stream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($stream)
        $errorBody = $reader.ReadToEnd()
        Write-Host "Error Details: $errorBody" -ForegroundColor Yellow
    }
} 