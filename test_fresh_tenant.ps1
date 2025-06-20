# Test Fresh Tenant Creation and Admin Login
Write-Host "Testing fresh tenant creation..." -ForegroundColor Cyan

# Create fresh tenant
$tenantPayload = @{
    name = "Fresh Test Company"
    subdomain = "freshtest"
    subscription_tier = "starter"
    admin_email = "admin@freshtest.com"
    admin_first_name = "Fresh"
    admin_last_name = "Admin"
    admin_password = "SecurePass123!"
} | ConvertTo-Json

Write-Host "Creating tenant 'freshtest'..."
try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tenant" -Method POST -Headers @{"Content-Type"="application/json"} -Body $tenantPayload
    Write-Host "SUCCESS: Tenant created!" -ForegroundColor Green
    Write-Host "Response: $($response | ConvertTo-Json -Depth 3)"
    
    # Now test admin login
    Write-Host "`nTesting admin login..." -ForegroundColor Yellow
    
    $loginPayload = @{
        email = "admin@freshtest.com"
        password = "SecurePass123!"
    } | ConvertTo-Json
    
    $loginHeaders = @{
        "X-Tenant-Subdomain" = "freshtest"
        "Content-Type" = "application/json"
    }
    
    try {
        $loginResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $loginHeaders -Body $loginPayload
        Write-Host "SUCCESS: Admin login worked!" -ForegroundColor Green
        Write-Host "Token: $($loginResponse.token.Substring(0,50))..."
    } catch {
        Write-Host "ERROR: Admin login failed - $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    }
    
} catch {
    Write-Host "ERROR: Tenant creation failed - $($_.Exception.Response.StatusCode)" -ForegroundColor Red
} 