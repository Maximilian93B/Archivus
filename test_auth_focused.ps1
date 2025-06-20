# Focused Authentication Test for Archivus
# Tests if Supabase Service Role Key resolves RLS issues

Write-Host "ARCHIVUS AUTHENTICATION FOCUSED TEST" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"

# Function to test endpoint with detailed error reporting
function Test-AuthEndpoint {
    param(
        [string]$Name,
        [string]$Url,
        [string]$Body
    )
    
    Write-Host "`nTesting: $Name" -ForegroundColor Yellow
    Write-Host "URL: $Url" -ForegroundColor Gray
    Write-Host "Body: $Body" -ForegroundColor Gray
    
    try {
        $response = Invoke-WebRequest -Uri $Url -Method POST -Body $Body -ContentType "application/json" -UseBasicParsing
        Write-Host "SUCCESS: $Name" -ForegroundColor Green
        Write-Host "Status: $($response.StatusCode)" -ForegroundColor Green
        Write-Host "Response: $($response.Content)" -ForegroundColor Green
        return $true
    }
    catch {
        Write-Host "ERROR: $Name" -ForegroundColor Red
        Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
        
        if ($_.Exception.Response) {
            $statusCode = $_.Exception.Response.StatusCode
            Write-Host "Status Code: $statusCode" -ForegroundColor Red
            
            try {
                $stream = $_.Exception.Response.GetResponseStream()
                $reader = New-Object System.IO.StreamReader($stream)
                $errorBody = $reader.ReadToEnd()
                Write-Host "Error Body: $errorBody" -ForegroundColor Yellow
            }
            catch {
                Write-Host "Could not read error response body" -ForegroundColor Yellow
            }
        }
        return $false
    }
}

# Test 1: Health Check
Write-Host "STEP 1: HEALTH CHECK" -ForegroundColor Magenta
try {
    $health = Invoke-WebRequest -Uri "$baseUrl/health" -Method GET -UseBasicParsing
    Write-Host "Health Check: PASS ($($health.StatusCode))" -ForegroundColor Green
}
catch {
    Write-Host "Health Check: FAIL - Server not running?" -ForegroundColor Red
    exit 1
}

# Test 2: Readiness Check
Write-Host "`nSTEP 2: READINESS CHECK" -ForegroundColor Magenta
try {
    $ready = Invoke-WebRequest -Uri "$baseUrl/ready" -Method GET -UseBasicParsing
    Write-Host "Readiness Check: PASS ($($ready.StatusCode))" -ForegroundColor Green
    Write-Host "Response: $($ready.Content)" -ForegroundColor Gray
}
catch {
    Write-Host "Readiness Check: FAIL" -ForegroundColor Red
}

# Test 3: User Registration (This should work with Service Role Key)
Write-Host "`nSTEP 3: USER REGISTRATION TEST" -ForegroundColor Magenta

$testUser = @{
    email = "testuser$(Get-Random -Maximum 9999)@archivus.test"
    password = "SecureTest123!"
    first_name = "Test"
    last_name = "User"
    tenant_name = "Test Company $(Get-Random -Maximum 99)"
    tenant_subdomain = "testco$(Get-Random -Maximum 99)"
} | ConvertTo-Json

$registrationSuccess = Test-AuthEndpoint "User Registration" "$baseUrl/api/v1/auth/register" $testUser

# Test 4: Login Test (only if registration succeeded)
if ($registrationSuccess) {
    Write-Host "`nSTEP 4: LOGIN TEST" -ForegroundColor Magenta
    
    $loginUser = @{
        email = ($testUser | ConvertFrom-Json).email
        password = "SecureTest123!"
    } | ConvertTo-Json
    
    Test-AuthEndpoint "User Login" "$baseUrl/api/v1/auth/login" $loginUser
}
else {
    Write-Host "`nSTEP 4: LOGIN TEST SKIPPED (Registration failed)" -ForegroundColor Yellow
}

# Test 5: Simple endpoints that don't require auth
Write-Host "`nSTEP 5: VALIDATION TOKEN ENDPOINT TEST" -ForegroundColor Magenta
try {
    $validate = Invoke-WebRequest -Uri "$baseUrl/api/v1/auth/validate" -Method GET -UseBasicParsing
    Write-Host "Validation endpoint accessible (expected to fail without token): $($validate.StatusCode)" -ForegroundColor Gray
}
catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "Validation endpoint properly requires authentication: PASS" -ForegroundColor Green
    }
    else {
        Write-Host "Validation endpoint error: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host "`nFOCUSED AUTHENTICATION TEST COMPLETE" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan

Write-Host "`nNEXT STEPS:" -ForegroundColor Yellow
Write-Host "1. If registration failed, check server logs for Supabase errors" -ForegroundColor White
Write-Host "2. Verify SUPABASE_SERVICE_KEY is properly loaded in server" -ForegroundColor White
Write-Host "3. Check Supabase dashboard for any auth configuration issues" -ForegroundColor White 