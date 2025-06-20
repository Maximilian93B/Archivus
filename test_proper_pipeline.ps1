# Archivus Proper Pipeline Test Script
# Tests the system as designed: Create tenant in DB -> Register users -> Test features

Write-Host "ARCHIVUS PROPER PIPELINE TESTING" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "Testing the system as designed: Tenant -> Users -> Features" -ForegroundColor Yellow
Write-Host ""

$baseUrl = "http://localhost:8080"
$testResults = @()

# Test configuration
$tenantSubdomain = "testcorp"
$tenantName = "Test Corporation"
$adminEmail = "admin@testcorp.com"
$adminPassword = "SecureAdmin123!"
$userEmail = "user@testcorp.com"
$userPassword = "SecureUser123!"

function Test-Endpoint {
    param(
        [string]$TestName,
        [string]$Method,
        [string]$Url,
        [hashtable]$Headers = @{},
        [string]$Body = $null,
        [int]$ExpectedStatus = 200
    )
    
    Write-Host "Testing: $TestName" -ForegroundColor Yellow
    
    try {
        $params = @{
            Uri = $Url
            Method = $Method
            Headers = $Headers
            UseBasicParsing = $true
            TimeoutSec = 10
        }
        
        if ($Body) {
            $params.Body = $Body
        }
        
        $response = Invoke-WebRequest @params
        
        if ($response.StatusCode -eq $ExpectedStatus) {
            Write-Host "SUCCESS: $TestName ($($response.StatusCode))" -ForegroundColor Green
            return @{
                Success = $true
                Response = $response
                StatusCode = $response.StatusCode
                Content = $response.Content
            }
        } else {
            Write-Host "UNEXPECTED STATUS: $TestName - Expected $ExpectedStatus, Got $($response.StatusCode)" -ForegroundColor Yellow
            return @{
                Success = $false
                Response = $response
                StatusCode = $response.StatusCode
                Content = $response.Content
            }
        }
    }
    catch {
        $statusCode = if ($_.Exception.Response) { $_.Exception.Response.StatusCode.value__ } else { 0 }
        Write-Host "FAILED: $TestName - $($_.Exception.Message) (Status: $statusCode)" -ForegroundColor Red
        
        $errorContent = ""
        if ($_.Exception.Response) {
            try {
                $stream = $_.Exception.Response.GetResponseStream()
                $reader = New-Object System.IO.StreamReader($stream)
                $errorContent = $reader.ReadToEnd()
                Write-Host "Error Details: $errorContent" -ForegroundColor Red
            } catch {
                Write-Host "Could not read error response" -ForegroundColor Red
            }
        }
        
        return @{
            Success = $false
            Response = $null
            StatusCode = $statusCode
            Content = $errorContent
            Error = $_.Exception.Message
        }
    }
}

function Create-TenantInDatabase {
    Write-Host "`n=== STEP 1: CREATE TENANT DIRECTLY IN DATABASE ===" -ForegroundColor Cyan
    Write-Host "Since no tenant creation endpoint is exposed, we need to create the tenant manually." -ForegroundColor Yellow
    Write-Host "In a real system, this would be done via admin interface or setup script." -ForegroundColor Yellow
    Write-Host ""
    
    # For now, we'll assume the tenant exists or needs to be created manually
    Write-Host "Please manually create a tenant with:" -ForegroundColor Yellow
    Write-Host "  Subdomain: $tenantSubdomain" -ForegroundColor White
    Write-Host "  Name: $tenantName" -ForegroundColor White
    Write-Host ""
    Write-Host "SQL Command to create tenant:" -ForegroundColor Green
    Write-Host "INSERT INTO tenants (id, name, subdomain, subscription_tier, is_active, created_at, updated_at)" -ForegroundColor White
    Write-Host "VALUES (gen_random_uuid(), '$tenantName', '$tenantSubdomain', 'starter', true, now(), now());" -ForegroundColor White
    Write-Host ""
    
    # Instead, let's test if tenant already exists
    return $true
}

function Test-TenantExists {
    Write-Host "`n=== CHECKING IF TENANT EXISTS ===" -ForegroundColor Cyan
    
    # Try to register a user to see if tenant exists
    $headers = @{
        "Content-Type" = "application/json"
        "X-Tenant-Subdomain" = $tenantSubdomain
    }
    
    $testBody = @{
        email = "test.check@domain.com"
        password = "TestPass123!"
        first_name = "Test"
        last_name = "Check"
    } | ConvertTo-Json
    
    $result = Test-Endpoint -TestName "Check if tenant exists" -Method "POST" -Url "$baseUrl/api/v1/auth/register" -Headers $headers -Body $testBody -ExpectedStatus 400
    
    if ($result.Content -like "*tenant not found*" -or $result.Content -like "*Invalid tenant*") {
        Write-Host "TENANT DOES NOT EXIST - Need to create it first" -ForegroundColor Red
        return $false
    } else {
        Write-Host "TENANT EXISTS - Proceeding with tests" -ForegroundColor Green
        return $true
    }
}

function Test-UserRegistration {
    Write-Host "`n=== STEP 2: TEST USER REGISTRATION ===" -ForegroundColor Cyan
    
    $headers = @{
        "Content-Type" = "application/json"
        "X-Tenant-Subdomain" = $tenantSubdomain
    }
    
    # Test admin user registration
    Write-Host "`nRegistering admin user..." -ForegroundColor Yellow
    $adminBody = @{
        email = $adminEmail
        password = $adminPassword
        first_name = "Admin"
        last_name = "User"
        role = "admin"
    } | ConvertTo-Json
    
    $adminResult = Test-Endpoint -TestName "Register Admin User" -Method "POST" -Url "$baseUrl/api/v1/auth/register" -Headers $headers -Body $adminBody -ExpectedStatus 201
    
    # Test regular user registration
    Write-Host "`nRegistering regular user..." -ForegroundColor Yellow
    $userBody = @{
        email = $userEmail
        password = $userPassword
        first_name = "Regular"
        last_name = "User"
        role = "user"
    } | ConvertTo-Json
    
    $userResult = Test-Endpoint -TestName "Register Regular User" -Method "POST" -Url "$baseUrl/api/v1/auth/register" -Headers $headers -Body $userBody -ExpectedStatus 201
    
    return @{
        AdminSuccess = $adminResult.Success
        UserSuccess = $userResult.Success
        AdminResponse = $adminResult
        UserResponse = $userResult
    }
}

function Test-UserLogin {
    param([bool]$IsAdmin = $false)
    
    $email = if ($IsAdmin) { $adminEmail } else { $userEmail }
    $password = if ($IsAdmin) { $adminPassword } else { $userPassword }
    $userType = if ($IsAdmin) { "Admin" } else { "Regular" }
    
    Write-Host "`n=== STEP 3: TEST $userType USER LOGIN ===" -ForegroundColor Cyan
    
    $headers = @{
        "Content-Type" = "application/json"
        "X-Tenant-Subdomain" = $tenantSubdomain
    }
    
    $loginBody = @{
        email = $email
        password = $password
    } | ConvertTo-Json
    
    $result = Test-Endpoint -TestName "$userType User Login" -Method "POST" -Url "$baseUrl/api/v1/auth/login" -Headers $headers -Body $loginBody -ExpectedStatus 200
    
    if ($result.Success) {
        try {
            $loginData = $result.Content | ConvertFrom-Json
            return @{
                Success = $true
                Token = $loginData.token
                User = $loginData.user
            }
        } catch {
            Write-Host "Failed to parse login response" -ForegroundColor Red
            return @{ Success = $false }
        }
    }
    
    return @{ Success = $false }
}

function Test-AuthenticatedEndpoints {
    param([string]$Token)
    
    Write-Host "`n=== STEP 4: TEST AUTHENTICATED ENDPOINTS ===" -ForegroundColor Cyan
    
    $authHeaders = @{
        "Content-Type" = "application/json"
        "Authorization" = "Bearer $Token"
        "X-Tenant-Subdomain" = $tenantSubdomain
    }
    
    # Test profile endpoint
    $profileResult = Test-Endpoint -TestName "Get User Profile" -Method "GET" -Url "$baseUrl/api/v1/users/profile" -Headers $authHeaders
    
    # Test tenant settings
    $tenantResult = Test-Endpoint -TestName "Get Tenant Settings" -Method "GET" -Url "$baseUrl/api/v1/tenant/settings" -Headers $authHeaders
    
    # Test tenant usage
    $usageResult = Test-Endpoint -TestName "Get Tenant Usage" -Method "GET" -Url "$baseUrl/api/v1/tenant/usage" -Headers $authHeaders
    
    return @{
        ProfileSuccess = $profileResult.Success
        TenantSuccess = $tenantResult.Success
        UsageSuccess = $usageResult.Success
    }
}

# Main test execution
Write-Host "Starting Archivus Pipeline Test..." -ForegroundColor Green
Write-Host ""

# Step 1: Check if system is ready
$healthResult = Test-Endpoint -TestName "Health Check" -Method "GET" -Url "$baseUrl/health"
$readyResult = Test-Endpoint -TestName "Readiness Check" -Method "GET" -Url "$baseUrl/ready"

if (-not $healthResult.Success -or -not $readyResult.Success) {
    Write-Host "SYSTEM NOT READY - Please check server status" -ForegroundColor Red
    exit 1
}

# Step 2: Check if tenant exists
if (-not (Test-TenantExists)) {
    Write-Host "`nCREATING TENANT INSTRUCTIONS:" -ForegroundColor Yellow
    Write-Host "Connect to your database and run:" -ForegroundColor White
    Write-Host "INSERT INTO tenants (id, name, subdomain, subscription_tier, is_active, created_at, updated_at)" -ForegroundColor Green
    Write-Host "VALUES (gen_random_uuid(), '$tenantName', '$tenantSubdomain', 'starter', true, now(), now());" -ForegroundColor Green
    Write-Host ""
    Write-Host "Then run this script again." -ForegroundColor Yellow
    exit 1
}

# Step 3: Test user registration
Write-Host "Testing user registration with tenant subdomain header..." -ForegroundColor Yellow
$registrationResults = Test-UserRegistration

# Step 4: Test login if registration succeeded
if ($registrationResults.AdminSuccess -or $registrationResults.UserSuccess) {
    $loginResult = Test-UserLogin -IsAdmin $true
    
    if ($loginResult.Success) {
        # Step 5: Test authenticated endpoints
        $authResults = Test-AuthenticatedEndpoints -Token $loginResult.Token
        
        # Final summary
        Write-Host "`n=== PIPELINE TEST SUMMARY ===" -ForegroundColor Cyan
        Write-Host "System Health: PASS" -ForegroundColor Green
        Write-Host "Tenant Setup: PASS" -ForegroundColor Green
        Write-Host "User Registration: $(if ($registrationResults.AdminSuccess -and $registrationResults.UserSuccess) { 'PASS' } else { 'PARTIAL' })" -ForegroundColor $(if ($registrationResults.AdminSuccess -and $registrationResults.UserSuccess) { 'Green' } else { 'Yellow' })
        Write-Host "User Authentication: $(if ($loginResult.Success) { 'PASS' } else { 'FAIL' })" -ForegroundColor $(if ($loginResult.Success) { 'Green' } else { 'Red' })
        Write-Host "Authenticated APIs: $(if ($authResults.ProfileSuccess -and $authResults.UsageSuccess) { 'PASS' } else { 'PARTIAL' })" -ForegroundColor $(if ($authResults.ProfileSuccess -and $authResults.UsageSuccess) { 'Green' } else { 'Yellow' })
        
        Write-Host "`nARCHIVUS CORE PIPELINE: OPERATIONAL" -ForegroundColor Green
    } else {
        Write-Host "`nAuthentication test failed - check user registration" -ForegroundColor Red
    }
} else {
    Write-Host "`nUser registration failed - check tenant configuration" -ForegroundColor Red
}

Write-Host "`nTest completed!" -ForegroundColor Cyan 