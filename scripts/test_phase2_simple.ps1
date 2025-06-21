# Archivus Phase 2 Simple Test
Write-Host "ARCHIVUS PHASE 2 SIMPLE TEST" -ForegroundColor Cyan
Write-Host "============================" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"

# Test 1: Health Check
Write-Host "1. Health Check..." -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "$baseUrl/health"
    Write-Host "   Status: $($health.status)" -ForegroundColor Green
} catch {
    Write-Host "   Failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test 2: Login
Write-Host "2. Login..." -ForegroundColor Yellow
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    Write-Host "   Login successful!" -ForegroundColor Green
    $token = $response.token
} catch {
    Write-Host "   Login failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test 3: Create test file
Write-Host "3. Creating test file..." -ForegroundColor Yellow
$testContent = "ARCHIVUS PHASE 2 TEST FILE`nCreated: $(Get-Date)`nTest content for pipeline validation"
$testFile = "simple_test.txt"
$testContent | Out-File -FilePath $testFile -Encoding UTF8
Write-Host "   Test file created" -ForegroundColor Green

# Test 4: Upload
Write-Host "4. File upload..." -ForegroundColor Yellow
try {
    $boundary = [System.Guid]::NewGuid().ToString()
    $fileBytes = [System.IO.File]::ReadAllBytes($testFile)
    $fileContent = [System.Text.Encoding]::UTF8.GetString($fileBytes)
    
    $uploadBody = @"
--$boundary
Content-Disposition: form-data; name="file"; filename="$testFile"
Content-Type: text/plain

$fileContent
--$boundary
Content-Disposition: form-data; name="title"

Simple Phase 2 Test
--$boundary--
"@
    
    $uploadHeaders = @{
        "Authorization" = "Bearer $token"
        "Content-Type" = "multipart/form-data; boundary=$boundary"
    }
    
    $uploadResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/upload" -Method POST -Headers $uploadHeaders -Body $uploadBody
    Write-Host "   Upload successful! ID: $($uploadResponse.id)" -ForegroundColor Green
    $documentId = $uploadResponse.id
    
} catch {
    Write-Host "   Upload failed: $($_.Exception.Message)" -ForegroundColor Red
    Remove-Item $testFile -ErrorAction SilentlyContinue
    exit 1
}

# Test 5: Download
Write-Host "5. File download..." -ForegroundColor Yellow
try {
    $downloadHeaders = @{ "Authorization" = "Bearer $token" }
    $downloadResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/documents/$documentId/download" -Method GET -Headers $downloadHeaders -UseBasicParsing
    Write-Host "   Download successful! Size: $($downloadResponse.Content.Length) bytes" -ForegroundColor Green
} catch {
    Write-Host "   Download failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Cleanup
Remove-Item $testFile -ErrorAction SilentlyContinue

Write-Host "`nPHASE 2 BASIC PIPELINE TEST COMPLETE!" -ForegroundColor Green 