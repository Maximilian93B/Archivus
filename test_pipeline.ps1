# Archivus Complete Pipeline Test Script
# Tests: Authentication -> Tenant Management -> Document Operations -> Search -> Analytics

Write-Host "🧪 ARCHIVUS COMPLETE PIPELINE TESTING" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan
Write-Host ""

$baseUrl = "http://localhost:8080"
$testResults = @()

# Test Helper Function
function Test-Endpoint {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Url,
        [hashtable]$Headers = @{},
        [string]$Body = $null,
        [bool]$ExpectSuccess = $true
    )
    
    Write-Host "🔍 Testing: $Name" -ForegroundColor Yellow
    
    try {
        $params = @{
            Uri = $Url
            Method = $Method
            Headers = $Headers
        }
        
        if ($Body) {
            $params.Body = $Body
            $params.ContentType = "application/json"
        }
        
        $response = Invoke-WebRequest @params
        $statusCode = $response.StatusCode
        $content = $response.Content
        
        if ($ExpectSuccess -and $statusCode -ge 200 -and $statusCode -lt 300) {
            Write-Host "✅ PASS: $Name (Status: $statusCode)" -ForegroundColor Green
            $script:testResults += @{Name = $Name; Status = "PASS"; Response = $content}
            return @{Success = $true; Data = $content; StatusCode = $statusCode}
        } elseif (-not $ExpectSuccess -and $statusCode -ge 400) {
            Write-Host "✅ PASS: $Name (Expected failure - Status: $statusCode)" -ForegroundColor Green
            $script:testResults += @{Name = $Name; Status = "PASS"; Response = $content}
            return @{Success = $true; Data = $content; StatusCode = $statusCode}
        } else {
            Write-Host "❌ FAIL: $Name (Status: $statusCode)" -ForegroundColor Red
            $script:testResults += @{Name = $Name; Status = "FAIL"; Response = $content}
            return @{Success = $false; Data = $content; StatusCode = $statusCode}
        }
    }
    catch {
        Write-Host "❌ ERROR: $Name - $($_.Exception.Message)" -ForegroundColor Red
        $script:testResults += @{Name = $Name; Status = "ERROR"; Response = $_.Exception.Message}
        return @{Success = $false; Data = $null; StatusCode = 0}
    }
}

# Phase 1: System Health Checks
Write-Host "📊 PHASE 1: SYSTEM HEALTH CHECKS" -ForegroundColor Magenta
Write-Host "================================" -ForegroundColor Magenta

Test-Endpoint "Health Check" "GET" "$baseUrl/health"
Test-Endpoint "Readiness Check" "GET" "$baseUrl/ready"

# Phase 2: Authentication Flow Testing
Write-Host "`n🔐 PHASE 2: AUTHENTICATION FLOW" -ForegroundColor Magenta
Write-Host "===============================" -ForegroundColor Magenta

# Test user registration (this should work now with the service role key)
$registerPayload = @{
    email = "test.user@archivus.test"
    password = "SecurePass123!"
    first_name = "Test"
    last_name = "User"
    tenant_name = "Test Company"
    tenant_subdomain = "testco"
} | ConvertTo-Json

$registerResult = Test-Endpoint "User Registration" "POST" "$baseUrl/api/v1/auth/register" @{} $registerPayload

# Test login
$loginPayload = @{
    email = "test.user@archivus.test"
    password = "SecurePass123!"
} | ConvertTo-Json

$loginResult = Test-Endpoint "User Login" "POST" "$baseUrl/api/v1/auth/login" @{} $loginPayload

# Extract token for authenticated requests
$authToken = $null
if ($loginResult.Success) {
    $loginData = $loginResult.Data | ConvertFrom-Json
    $authToken = $loginData.token
    Write-Host "🔑 Auth Token obtained for subsequent tests" -ForegroundColor Green
}

# Phase 3: Authenticated Operations
if ($authToken) {
    Write-Host "`n👤 PHASE 3: AUTHENTICATED OPERATIONS" -ForegroundColor Magenta
    Write-Host "===================================" -ForegroundColor Magenta
    
    $authHeaders = @{
        "Authorization" = "Bearer $authToken"
    }
    
    # Test profile access
    Test-Endpoint "Get User Profile" "GET" "$baseUrl/api/v1/users/profile" $authHeaders
    
    # Test tenant settings
    Test-Endpoint "Get Tenant Settings" "GET" "$baseUrl/api/v1/tenant/settings" $authHeaders
    Test-Endpoint "Get Tenant Usage" "GET" "$baseUrl/api/v1/tenant/usage" $authHeaders
    
    # Phase 4: Document Management Operations
    Write-Host "`n📁 PHASE 4: DOCUMENT MANAGEMENT" -ForegroundColor Magenta
    Write-Host "===============================" -ForegroundColor Magenta
    
    # Test folder creation
    $folderPayload = @{
        name = "Test Documents"
        description = "Folder for testing document operations"
    } | ConvertTo-Json
    
    $folderResult = Test-Endpoint "Create Folder" "POST" "$baseUrl/api/v1/folders" $authHeaders $folderPayload
    
    # Test category creation
    $categoryPayload = @{
        name = "Financial"
        description = "Financial documents and reports"
        color = "#3B82F6"
    } | ConvertTo-Json
    
    Test-Endpoint "Create Category" "POST" "$baseUrl/api/v1/categories" $authHeaders $categoryPayload
    
    # Test tag creation
    $tagPayload = @{
        name = "important"
        color = "#EF4444"
    } | ConvertTo-Json
    
    Test-Endpoint "Create Tag" "POST" "$baseUrl/api/v1/tags" $authHeaders $tagPayload
    
    # Test document listing (should be empty initially)
    Test-Endpoint "List Documents" "GET" "$baseUrl/api/v1/documents/" $authHeaders
    Test-Endpoint "List Folders" "GET" "$baseUrl/api/v1/folders" $authHeaders
    Test-Endpoint "List Categories" "GET" "$baseUrl/api/v1/categories" $authHeaders
    Test-Endpoint "List Tags" "GET" "$baseUrl/api/v1/tags" $authHeaders
}

# Phase 5: Search and Analytics
Write-Host "`n🔍 PHASE 5: SEARCH & ANALYTICS" -ForegroundColor Magenta
Write-Host "=============================" -ForegroundColor Magenta

if ($authToken) {
    Test-Endpoint "Search Documents" "GET" "$baseUrl/api/v1/documents/search?q=test" $authHeaders
    Test-Endpoint "Get Popular Tags" "GET" "$baseUrl/api/v1/tags/popular" $authHeaders
    Test-Endpoint "Get System Categories" "GET" "$baseUrl/api/v1/categories/system" $authHeaders
}

# Test Results Summary
Write-Host "`n📊 TEST RESULTS SUMMARY" -ForegroundColor Cyan
Write-Host "======================" -ForegroundColor Cyan

$passCount = ($testResults | Where-Object { $_.Status -eq "PASS" }).Count
$failCount = ($testResults | Where-Object { $_.Status -eq "FAIL" }).Count
$errorCount = ($testResults | Where-Object { $_.Status -eq "ERROR" }).Count
$totalCount = $testResults.Count

Write-Host "Total Tests: $totalCount" -ForegroundColor White
Write-Host "✅ Passed: $passCount" -ForegroundColor Green
Write-Host "❌ Failed: $failCount" -ForegroundColor Red
Write-Host "⚠️ Errors: $errorCount" -ForegroundColor Yellow

if ($failCount -eq 0 -and $errorCount -eq 0) {
    Write-Host "`n🎉 ALL TESTS PASSED! Archivus pipeline is fully operational!" -ForegroundColor Green
} else {
    Write-Host "`n⚠️ Some tests failed. Check the details above." -ForegroundColor Yellow
}

Write-Host "`n🚀 Phase 2 Core Features Testing Complete!" -ForegroundColor Cyan 