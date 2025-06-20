# Test Debug Tenant Creation  
Write-Host "Creating debug tenant to trace Supabase error..." -ForegroundColor Yellow

$tenantPayload = @{
    name = "Debug Test Company"
    subdomain = "debugtest"
    subscription_tier = "starter"
    admin_email = "admin@debugtest.com"
    admin_first_name = "Debug"
    admin_last_name = "Admin"
    admin_password = "SecurePass123!"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tenant" -Method POST -Headers @{"Content-Type"="application/json"} -Body $tenantPayload
    Write-Host "SUCCESS: Tenant created - $($response.tenant.subdomain)" -ForegroundColor Green
    Write-Host "Admin User ID: $($response.admin.id)" -ForegroundColor Cyan
    if ($response.admin.id -eq "00000000-0000-0000-0000-000000000000") {
        Write-Host "WARNING: Admin user has zero UUID - Supabase Auth failed!" -ForegroundColor Red
    }
} catch {
    Write-Host "ERROR: Tenant creation failed - $($_.Exception.Response.StatusCode)" -ForegroundColor Red
}