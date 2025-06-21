# Quick Background Worker Test
Write-Host "QUICK BACKGROUND WORKER TEST" -ForegroundColor Cyan
Write-Host "============================" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"
$scriptDir = $PSScriptRoot

# Test 1: Start Worker
Write-Host "1. Starting Worker..." -ForegroundColor Yellow
$workerPath = Join-Path (Split-Path $scriptDir) "archivus-worker.exe"

if (-not (Test-Path $workerPath)) {
    Write-Host "   Building worker..." -ForegroundColor Yellow
    Push-Location (Split-Path $scriptDir)
    $env:CGO_ENABLED=0
    go build -o archivus-worker.exe ./cmd/worker
    Pop-Location
}

$workerProcess = Start-Process -FilePath $workerPath -PassThru -WindowStyle Hidden
Write-Host "   Worker started (PID: $($workerProcess.Id))" -ForegroundColor Green
Start-Sleep -Seconds 3

# Test 2: Login
Write-Host "2. Login..." -ForegroundColor Yellow
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    Write-Host "   Success!" -ForegroundColor Green
    $token = $response.token
} catch {
    Write-Host "   Failed: $($_.Exception.Message)" -ForegroundColor Red
    Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
    exit 1
}

# Test 3: Upload
Write-Host "3. Upload test file..." -ForegroundColor Yellow
$testContent = "WORKER TEST`nCreated: $(Get-Date)"
$testFile = Join-Path $scriptDir "worker_test.txt"
$testContent | Out-File -FilePath $testFile -Encoding UTF8

try {
    $boundary = [System.Guid]::NewGuid().ToString()
    $fileBytes = [System.IO.File]::ReadAllBytes($testFile)
    $fileContent = [System.Text.Encoding]::UTF8.GetString($fileBytes)
    
    $uploadBody = @"
--$boundary
Content-Disposition: form-data; name="file"; filename="worker_test.txt"
Content-Type: text/plain

$fileContent
--$boundary
Content-Disposition: form-data; name="title"

Worker Test
--$boundary--
"@
    
    $uploadHeaders = @{
        "Authorization" = "Bearer $token"
        "Content-Type" = "multipart/form-data; boundary=$boundary"
    }
    
    $uploadResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/upload" -Method POST -Headers $uploadHeaders -Body $uploadBody
    Write-Host "   Upload success! ID: $($uploadResponse.id)" -ForegroundColor Green
    $documentId = $uploadResponse.id
    
} catch {
    Write-Host "   Upload failed: $($_.Exception.Message)" -ForegroundColor Red
    Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
    Remove-Item $testFile -ErrorAction SilentlyContinue
    exit 1
}

# Test 4: Monitor Processing
Write-Host "4. Monitor processing..." -ForegroundColor Yellow
$authHeaders = @{ "Authorization" = "Bearer $token" }

for ($i = 1; $i -le 10; $i++) {
    Start-Sleep -Seconds 2
    try {
        $document = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/$documentId" -Method GET -Headers $authHeaders
        Write-Host "   Status: $($document.status)" -ForegroundColor Gray
        
        if ($document.status -ne "pending") {
            Write-Host "   Processing complete!" -ForegroundColor Green
            break
        }
    } catch {
        Write-Host "   Check failed" -ForegroundColor Red
    }
}

# Test 5: Download
Write-Host "5. Test download..." -ForegroundColor Yellow
try {
    $downloadResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/documents/$documentId/download" -Method GET -Headers $authHeaders -UseBasicParsing
    Write-Host "   Download success!" -ForegroundColor Green
} catch {
    Write-Host "   Download failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Cleanup
Write-Host "6. Cleanup..." -ForegroundColor Yellow
Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
Remove-Item $testFile -ErrorAction SilentlyContinue
Write-Host "   Complete!" -ForegroundColor Green

Write-Host "`nQUICK WORKER TEST COMPLETE!" -ForegroundColor Green 