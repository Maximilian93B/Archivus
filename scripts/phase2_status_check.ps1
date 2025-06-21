# Archivus Phase 2 Status Check
Write-Host "ARCHIVUS PHASE 2 STATUS CHECK" -ForegroundColor Cyan
Write-Host "=============================" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"

Write-Host ""
Write-Host "Checking Phase 2 Implementation Status..." -ForegroundColor Yellow
Write-Host ""

# Check 1: Server Status
Write-Host "1. Server Health:" -ForegroundColor White
try {
    $health = Invoke-RestMethod -Uri "$baseUrl/health"
    Write-Host "   Status: $($health.status)" -ForegroundColor Green
    $serverRunning = $true
} catch {
    Write-Host "   Status: NOT RUNNING" -ForegroundColor Red
    $serverRunning = $false
}

# Check 2: Worker Binary
Write-Host "2. Background Worker Binary:" -ForegroundColor White
$workerPath = Join-Path (Split-Path $PSScriptRoot) "archivus-worker.exe"
if (Test-Path $workerPath) {
    $workerSize = (Get-Item $workerPath).Length / 1MB
    Write-Host "   Status: BUILT ($([math]::Round($workerSize, 1)) MB)" -ForegroundColor Green
    $workerBuilt = $true
} else {
    Write-Host "   Status: NOT BUILT" -ForegroundColor Red
    $workerBuilt = $false
}

# Check 3: Authentication (if server running)
Write-Host "3. Authentication System:" -ForegroundColor White
if ($serverRunning) {
    try {
        $headers = @{ 
            "Content-Type" = "application/json"
            "X-Tenant-Subdomain" = "testdebug123" 
        }
        $body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'
        $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method POST -Headers $headers -Body $body
        Write-Host "   Status: WORKING" -ForegroundColor Green
        $authWorking = $true
        $token = $response.token
    } catch {
        Write-Host "   Status: FAILED" -ForegroundColor Red
        $authWorking = $false
    }
} else {
    Write-Host "   Status: CANNOT TEST (server not running)" -ForegroundColor Yellow
    $authWorking = $false
}

# Check 4: Document Upload (if auth working)
Write-Host "4. File Upload System:" -ForegroundColor White
if ($authWorking) {
    try {
        $testContent = "STATUS CHECK TEST"
        $testFile = Join-Path $PSScriptRoot "status_test.txt"
        $testContent | Out-File -FilePath $testFile -Encoding UTF8
        
        $boundary = [System.Guid]::NewGuid().ToString()
        $fileBytes = [System.IO.File]::ReadAllBytes($testFile)
        $fileContent = [System.Text.Encoding]::UTF8.GetString($fileBytes)
        
        $uploadBody = @"
--$boundary
Content-Disposition: form-data; name="file"; filename="status_test.txt"
Content-Type: text/plain

$fileContent
--$boundary
Content-Disposition: form-data; name="title"

Status Check Test
--$boundary--
"@
        
        $uploadHeaders = @{
            "Authorization" = "Bearer $token"
            "Content-Type" = "multipart/form-data; boundary=$boundary"
        }
        
        $uploadResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/upload" -Method POST -Headers $uploadHeaders -Body $uploadBody
        Write-Host "   Status: WORKING" -ForegroundColor Green
        $uploadWorking = $true
        $testDocId = $uploadResponse.id
        
        Remove-Item $testFile -ErrorAction SilentlyContinue
    } catch {
        Write-Host "   Status: FAILED" -ForegroundColor Red
        $uploadWorking = $false
        Remove-Item $testFile -ErrorAction SilentlyContinue
    }
} else {
    Write-Host "   Status: CANNOT TEST (auth not working)" -ForegroundColor Yellow
    $uploadWorking = $false
}

# Check 5: Download System (if upload working)
Write-Host "5. File Download System:" -ForegroundColor White
if ($uploadWorking) {
    try {
        $authHeaders = @{ "Authorization" = "Bearer $token" }
        $downloadResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/documents/$testDocId/download" -Method GET -Headers $authHeaders -UseBasicParsing
        if ($downloadResponse.StatusCode -eq 200) {
            Write-Host "   Status: WORKING" -ForegroundColor Green
            $downloadWorking = $true
        } else {
            Write-Host "   Status: FAILED" -ForegroundColor Red
            $downloadWorking = $false
        }
    } catch {
        Write-Host "   Status: FAILED" -ForegroundColor Red
        $downloadWorking = $false
    }
} else {
    Write-Host "   Status: CANNOT TEST (upload not working)" -ForegroundColor Yellow
    $downloadWorking = $false
}

# Check 6: Background Worker Functionality
Write-Host "6. Background Worker System:" -ForegroundColor White
if ($workerBuilt) {
    Write-Host "   Status: READY (binary built, can be tested)" -ForegroundColor Green
    $workerReady = $true
} else {
    Write-Host "   Status: NOT READY (binary not built)" -ForegroundColor Red
    $workerReady = $false
}

Write-Host ""
Write-Host "="*50 -ForegroundColor Cyan
Write-Host "PHASE 2 STATUS SUMMARY" -ForegroundColor Green
Write-Host "="*50 -ForegroundColor Cyan
Write-Host ""

# Calculate overall status
$components = @(
    @{ name = "Server Health"; status = $serverRunning },
    @{ name = "Worker Binary"; status = $workerBuilt },
    @{ name = "Authentication"; status = $authWorking },
    @{ name = "File Upload"; status = $uploadWorking },
    @{ name = "File Download"; status = $downloadWorking },
    @{ name = "Background Worker"; status = $workerReady }
)

$workingCount = ($components | Where-Object { $_.status }).Count
$totalCount = $components.Count
$successRate = [math]::Round(($workingCount / $totalCount) * 100, 1)

Write-Host "Overall Status: $workingCount/$totalCount components working ($successRate%)" -ForegroundColor White
Write-Host ""

foreach ($component in $components) {
    $status = if ($component.status) { "WORKING" } else { "NEEDS ATTENTION" }
    $color = if ($component.status) { "Green" } else { "Red" }
    Write-Host "  $($component.name): $status" -ForegroundColor $color
}

Write-Host ""

if ($successRate -ge 90) {
    Write-Host "PHASE 2 STATUS: EXCELLENT - Ready for Phase 3!" -ForegroundColor Green
} elseif ($successRate -ge 75) {
    Write-Host "PHASE 2 STATUS: GOOD - Minor issues to resolve" -ForegroundColor Yellow
} elseif ($successRate -ge 50) {
    Write-Host "PHASE 2 STATUS: NEEDS WORK - Several components need attention" -ForegroundColor Yellow
} else {
    Write-Host "PHASE 2 STATUS: CRITICAL - Major issues to resolve" -ForegroundColor Red
}

Write-Host ""
Write-Host "Next Steps:" -ForegroundColor White

if (-not $serverRunning) {
    Write-Host "  1. Start the server: go run ./cmd/server" -ForegroundColor Yellow
}

if (-not $workerBuilt) {
    Write-Host "  2. Build worker: go build -o archivus-worker.exe ./cmd/worker" -ForegroundColor Yellow
}

if ($successRate -ge 75) {
    Write-Host "  • Run comprehensive tests: ./test_phase2_complete.ps1" -ForegroundColor Green
    Write-Host "  • Test background workers: ./test_worker_comprehensive.ps1" -ForegroundColor Green
    Write-Host "  • Ready to plan Phase 3 (AI Integration)" -ForegroundColor Green
}

Write-Host "" 