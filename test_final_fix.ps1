# Test Final AdminCreateUser Fix
Write-Host "Testing our AdminCreateUser fix..." -ForegroundColor Cyan

$timestamp = Get-Date -Format "MMddHHmmss"
$subdomain = "finaltest$timestamp"

# Create tenant with unique timestamp
$tenantPayload = @{
    name = "Final Test Company"
    subdomain = $subdomain
    subscription_tier = "starter"
    admin_email = "admin@$subdomain.com"
    admin_first_name = "Final"
    admin_last_name = "Admin"
    admin_password = "SecurePass123!"
} | ConvertTo-Json

Write-Host "Creating tenant '$subdomain'..."
try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tenant" -Method POST -Headers @{"Content-Type"="application/json"} -Body $tenantPayload
    Write-Host "SUCCESS: Tenant created!" -ForegroundColor Green
    Write-Host "Tenant ID: $($response.tenant.id)"
    Write-Host "Admin User ID: $($response.admin.id)" -ForegroundColor Cyan
    
    if ($response.admin.id -eq "00000000-0000-0000-0000-000000000000") {
        Write-Host "WARNING: Zero UUID - Supabase Auth still failing!" -ForegroundColor Red
    } else {
        Write-Host "GOOD: Non-zero UUID - UserService integration working!" -ForegroundColor Green
        
        # Now test admin login
        Write-Host "`nTesting admin login..." -ForegroundColor Yellow
        
        $loginPayload = @{
            email = "admin@$subdomain.com"
            password = "SecurePass123!"
        } | ConvertTo-Json
        
        $loginHeaders = @{
            "X-Tenant-Subdomain" = $subdomain
            "Content-Type" = "application/json"
        }
        
        try {
            $loginResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $loginHeaders -Body $loginPayload
            Write-Host "🎉 SUCCESS: Admin login worked! Authentication FIXED!" -ForegroundColor Green
            Write-Host "Token: $($loginResponse.token.Substring(0,30))..." -ForegroundColor Cyan
        } catch {
            Write-Host "❌ ERROR: Admin login failed - $($_.Exception.Response.StatusCode)" -ForegroundColor Red
            Write-Host "This means Supabase Auth user creation is still not working properly." -ForegroundColor Yellow
        }
    }
} catch {
    Write-Host "ERROR: Tenant creation failed - $($_.Exception.Response.StatusCode)" -ForegroundColor Red
} 