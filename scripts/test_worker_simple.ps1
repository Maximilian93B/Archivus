# Archivus Simple Background Worker Test
Write-Host "ARCHIVUS SIMPLE BACKGROUND WORKER TEST" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"

# Test 1: Start Worker
Write-Host "1. Starting Background Worker..." -ForegroundColor Yellow
$workerPath = "..\archivus-worker.exe"
if (-not (Test-Path $workerPath)) {
    Write-Host "   Worker binary not found. Building..." -ForegroundColor Yellow
    Set-Location ..
    $env:CGO_ENABLED=0
    go build -o archivus-worker.exe ./cmd/worker
    Set-Location scripts
}

$workerProcess = Start-Process -FilePath $workerPath -PassThru -WindowStyle Hidden
Write-Host "   Worker started with PID: $($workerProcess.Id)" -ForegroundColor Green
Start-Sleep -Seconds 3

# Test 2: Login
Write-Host "2. Authentication..." -ForegroundColor Yellow
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
    Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
    exit 1
}

# Test 3: Upload Test File
Write-Host "3. Uploading test file..." -ForegroundColor Yellow
$testContent = "WORKER TEST FILE`nCreated: $(Get-Date)`nThis file will trigger background processing jobs."
$testFile = "worker_test.txt"
$testContent | Out-File -FilePath $testFile -Encoding UTF8

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

Background Worker Test
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
    Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
    Remove-Item $testFile -ErrorAction SilentlyContinue
    exit 1
}

# Test 4: Monitor Processing
Write-Host "4. Monitoring background processing..." -ForegroundColor Yellow
$authHeaders = @{ "Authorization" = "Bearer $token" }
$maxWait = 20
$waited = 0

while ($waited -lt $maxWait) {
    Start-Sleep -Seconds 2
    $waited += 2
    
    try {
        $document = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/$documentId" -Method GET -Headers $authHeaders
        Write-Host "   Status: $($document.status) (waited $waited seconds)" -ForegroundColor Gray
        
        if ($document.status -ne "pending") {
            Write-Host "   Processing complete!" -ForegroundColor Green
            break
        }
    } catch {
        Write-Host "   Error checking status" -ForegroundColor Red
    }
}

# Test 5: Verify Results
Write-Host "5. Verifying results..." -ForegroundColor Yellow
try {
    $document = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/$documentId" -Method GET -Headers $authHeaders
    Write-Host "   Final Status: $($document.status)" -ForegroundColor White
    Write-Host "   File Size: $($document.file_size) bytes" -ForegroundColor White
    
    # Test download
    $downloadResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/documents/$documentId/download" -Method GET -Headers $authHeaders -UseBasicParsing
    if ($downloadResponse.StatusCode -eq 200) {
        Write-Host "   Download: SUCCESS" -ForegroundColor Green
    }
} catch {
    Write-Host "   Verification failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 6: Check Worker Health
Write-Host "6. Checking worker health..." -ForegroundColor Yellow
if (Get-Process -Id $workerProcess.Id -ErrorAction SilentlyContinue) {
    Write-Host "   Worker is still running" -ForegroundColor Green
} else {
    Write-Host "   Worker has stopped" -ForegroundColor Red
}

# Cleanup
Write-Host "7. Cleaning up..." -ForegroundColor Yellow
Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
Remove-Item $testFile -ErrorAction SilentlyContinue
Write-Host "   Cleanup complete" -ForegroundColor Green

Write-Host "`nSIMPLE WORKER TEST COMPLETE!" -ForegroundColor Green 