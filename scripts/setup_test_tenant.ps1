#!/usr/bin/env pwsh

# Setup Test Tenant and Admin User for Sprint 2 Testing
Write-Host "Setting up test tenant and admin user..." -ForegroundColor Cyan

$BASE_URL = "http://localhost:8080"

# Tenant creation payload
$tenantPayload = @{
    name = "Archivus Test Company"
    subdomain = "archivus"
    subscription_tier = "starter"
    admin_email = "admin@archivus.com"
    admin_first_name = "Admin"
    admin_last_name = "User"
    admin_password = "admin123"
} | ConvertTo-Json

try {
    Write-Host "Creating tenant 'archivus' with admin user..." -ForegroundColor Yellow
    
    $response = Invoke-RestMethod -Uri "$BASE_URL/api/v1/tenant" -Method POST -Headers @{"Content-Type"="application/json"} -Body $tenantPayload
    
    if ($response.tenant -and $response.admin) {
        Write-Host "SUCCESS: Tenant created successfully!" -ForegroundColor Green
        Write-Host "   Tenant: $($response.tenant.name) ($($response.tenant.subdomain))" -ForegroundColor White
        Write-Host "   Admin: $($response.admin.email)" -ForegroundColor White
        Write-Host "   Admin ID: $($response.admin.id)" -ForegroundColor White
        
        # Test login immediately
        Write-Host "`nTesting admin login..." -ForegroundColor Yellow
        
        $loginData = @{
            email = "admin@archivus.com"
            password = "admin123"
        } | ConvertTo-Json
        
        $headers = @{
            "X-Tenant-Subdomain" = "archivus"
            "Content-Type" = "application/json"
        }
        
        $loginResponse = Invoke-RestMethod -Uri "$BASE_URL/api/v1/auth/login" -Method POST -Body $loginData -Headers $headers
        
        if ($loginResponse.token) {
            Write-Host "Admin login successful!" -ForegroundColor Green
            Write-Host "   Token length: $($loginResponse.token.Length) characters" -ForegroundColor White
            
            Write-Host "`nSetup Complete! Ready for Sprint 2 testing." -ForegroundColor Green
            Write-Host "You can now run: ./scripts/test_sprint2_simple.ps1" -ForegroundColor Cyan
        } else {
            Write-Host "WARNING: Tenant created but login failed" -ForegroundColor Yellow
        }
        
    } else {
        Write-Host "ERROR: Tenant creation failed - invalid response" -ForegroundColor Red
    }
    
} catch {
    $errorDetails = $_.Exception.Response
    if ($errorDetails) {
        $statusCode = $errorDetails.StatusCode
        $statusDescription = $errorDetails.StatusDescription
        Write-Host "ERROR: HTTP $statusCode - $statusDescription" -ForegroundColor Red
        
        # Try to get more details from response body
        try {
            $stream = $errorDetails.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($stream)
            $responseBody = $reader.ReadToEnd()
            Write-Host "   Details: $responseBody" -ForegroundColor Red
        } catch {
            Write-Host "   Could not read error details" -ForegroundColor Yellow
        }
    } else {
        Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    }
    
    Write-Host "`nNOTE: If tenant already exists, you can proceed with testing." -ForegroundColor Yellow
} 