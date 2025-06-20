# Comprehensive Authentication Debug Script
Write-Host "ARCHIVUS AUTHENTICATION DEBUG" -ForegroundColor Cyan
Write-Host "=============================" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"
$headers = @{
    'Content-Type' = 'application/json'
    'X-Tenant-Subdomain' = 'testcorp'
}

# Test 1: Health Check
Write-Host "`n1. TESTING SYSTEM HEALTH" -ForegroundColor Yellow
try {
    $health = Invoke-WebRequest -Uri "$baseUrl/health" -UseBasicParsing
    Write-Host "✅ Health: $($health.StatusCode)" -ForegroundColor Green
} catch {
    Write-Host "❌ Health check failed" -ForegroundColor Red
}

# Test 2: Create a brand new user
Write-Host "`n2. CREATING NEW TEST USER" -ForegroundColor Yellow
$newUserEmail = "debug.$(Get-Date -Format 'yyyyMMddHHmmss')@testcorp.com"
$newUserBody = @{
    email = $newUserEmail
    password = 'DebugPass123!'
    first_name = 'Debug'
    last_name = 'User'
    role = 'user'
} | ConvertTo-Json

Write-Host "Creating user: $newUserEmail" -ForegroundColor Gray

try {
    $regResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/auth/register" -Method POST -Body $newUserBody -Headers $headers -UseBasicParsing
    Write-Host "✅ Registration SUCCESS: $($regResponse.StatusCode)" -ForegroundColor Green
    
    $regData = $regResponse.Content | ConvertFrom-Json
    Write-Host "User ID: $($regData.id)" -ForegroundColor Cyan
    Write-Host "User Email: $($regData.email)" -ForegroundColor Cyan
    
    # Test 3: Immediately try to login with the new user
    Write-Host "`n3. TESTING LOGIN WITH NEW USER" -ForegroundColor Yellow
    
    $loginBody = @{
        email = $newUserEmail
        password = 'DebugPass123!'
    } | ConvertTo-Json
    
    try {
        $loginResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/auth/login" -Method POST -Body $loginBody -Headers $headers -UseBasicParsing
        Write-Host "✅ LOGIN SUCCESS: $($loginResponse.StatusCode)" -ForegroundColor Green
        
        $loginData = $loginResponse.Content | ConvertFrom-Json
        Write-Host "Token received: $($loginData.token.Substring(0,20))..." -ForegroundColor Cyan
        Write-Host "🎉 AUTHENTICATION WORKING!" -ForegroundColor Green
        
    } catch {
        Write-Host "❌ LOGIN FAILED: $($_.Exception.Message)" -ForegroundColor Red
        
        if ($_.Exception.Response) {
            $stream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($stream)
            $errorBody = $reader.ReadToEnd()
            Write-Host "Login Error Details: $errorBody" -ForegroundColor Yellow
        }
    }
    
} catch {
    Write-Host "❌ REGISTRATION FAILED: $($_.Exception.Message)" -ForegroundColor Red
    
    if ($_.Exception.Response) {
        $stream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($stream)
        $errorBody = $reader.ReadToEnd()
        Write-Host "Registration Error Details: $errorBody" -ForegroundColor Yellow
    }
}

# Test 4: Try the old admin user again
Write-Host "`n4. TESTING OLD ADMIN USER" -ForegroundColor Yellow
$adminBody = @{
    email = 'admin@testcorp.com'
    password = 'SecureAdmin123!'
} | ConvertTo-Json

try {
    $adminResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/auth/login" -Method POST -Body $adminBody -Headers $headers -UseBasicParsing
    Write-Host "✅ ADMIN LOGIN SUCCESS" -ForegroundColor Green
} catch {
    Write-Host "❌ ADMIN LOGIN FAILED: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`n=== DEBUG COMPLETE ===" -ForegroundColor Cyan 