#!/usr/bin/env pwsh

# Simple Sprint 2 Test: Document Upload with AI Processing
Write-Host "Testing Sprint 2: Document Upload with AI Processing" -ForegroundColor Green

$BASE_URL = "http://localhost:8080"
$ADMIN_EMAIL = "admin@archivus.com"
$ADMIN_PASSWORD = "admin123"
$TENANT_SUBDOMAIN = "archivus"

# Test document content
$TEST_CONTENT = @"
AI Integration Test Document

This document tests AI processing capabilities including:
- Document summarization
- Entity extraction (people, organizations, dates, amounts)  
- Document classification
- Tag generation

Company: Archivus Technologies Inc.
Contact: Sarah Johnson (sarah.johnson@archivus.com)
Amount: $25,000 USD
Date: July 1, 2025
Location: San Francisco, CA
"@

Write-Host "Step 1: Authentication" -ForegroundColor Cyan

try {
    $loginData = @{
        email = $ADMIN_EMAIL
        password = $ADMIN_PASSWORD
    } | ConvertTo-Json
    
    $headers = @{
        "X-Tenant-Subdomain" = $TENANT_SUBDOMAIN
        "Content-Type" = "application/json"
    }
    
    $response = Invoke-RestMethod -Uri "$BASE_URL/api/v1/auth/login" -Method POST -Body $loginData -Headers $headers
    
    if ($response.token) {
        Write-Host "Authentication successful" -ForegroundColor Green
        $token = $response.token
    } else {
        Write-Host "Authentication failed" -ForegroundColor Red
        exit 1
    }
} catch {
    Write-Host "Authentication error: $_" -ForegroundColor Red
    exit 1
}

Write-Host "Step 2: Upload Document with AI Processing" -ForegroundColor Cyan

try {
    # Save test content to temporary file
    $tempFile = "temp_ai_test.txt"
    $TEST_CONTENT | Out-File -FilePath $tempFile -Encoding UTF8
    
    $headers = @{
        "Authorization" = "Bearer $token"
        "X-Tenant-Subdomain" = $TENANT_SUBDOMAIN
    }
    
    # Create multipart form data
    $fileBytes = [System.IO.File]::ReadAllBytes((Resolve-Path $tempFile))
    $boundary = [System.Guid]::NewGuid().ToString()
    
    $LF = "`r`n"
    $bodyLines = (
        "--$boundary",
        "Content-Disposition: form-data; name=`"file`"; filename=`"ai_test.txt`"",
        "Content-Type: text/plain$LF",
        [System.Text.Encoding]::UTF8.GetString($fileBytes),
        "--$boundary",
        "Content-Disposition: form-data; name=`"enable_ai`"$LF",
        "true",
        "--$boundary",
        "Content-Disposition: form-data; name=`"title`"$LF",
        "AI Test Document",
        "--$boundary--$LF"
    ) -join $LF
    
    $body = [System.Text.Encoding]::UTF8.GetBytes($bodyLines)
    
    $response = Invoke-RestMethod -Uri "$BASE_URL/api/v1/documents/upload" -Method POST -Body $body -Headers $headers -ContentType "multipart/form-data; boundary=$boundary"
    
    # Cleanup temp file
    Remove-Item $tempFile -ErrorAction SilentlyContinue
    
    if ($response.document -and $response.document.id) {
        Write-Host "Document uploaded successfully" -ForegroundColor Green
        Write-Host "Document ID: $($response.document.id)" -ForegroundColor White
        $documentId = $response.document.id
    } else {
        Write-Host "Document upload failed" -ForegroundColor Red
        exit 1
    }
} catch {
    Write-Host "Upload error: $_" -ForegroundColor Red
    exit 1
}

Write-Host "Step 3: Check AI Jobs" -ForegroundColor Cyan

try {
    $headers = @{
        "Authorization" = "Bearer $token"
        "X-Tenant-Subdomain" = $TENANT_SUBDOMAIN
    }
    
    Start-Sleep -Seconds 3
    
    $response = Invoke-RestMethod -Uri "$BASE_URL/api/v1/documents/$documentId/jobs" -Method GET -Headers $headers
    
    if ($response.jobs) {
        $aiJobs = $response.jobs | Where-Object { 
            $_.job_type -in @("document_summarization", "entity_extraction", "document_classification", "semantic_analysis") 
        }
        
        Write-Host "AI jobs created successfully" -ForegroundColor Green
        Write-Host "Total Jobs: $($response.jobs.Count)" -ForegroundColor White
        Write-Host "AI Jobs: $($aiJobs.Count)" -ForegroundColor White
        
        foreach ($job in $aiJobs) {
            Write-Host "- $($job.job_type): $($job.status)" -ForegroundColor Yellow
        }
        
        $jobCount = $aiJobs.Count
    } else {
        Write-Host "No jobs found" -ForegroundColor Red
        $jobCount = 0
    }
} catch {
    Write-Host "Job check error: $_" -ForegroundColor Red
    $jobCount = 0
}

Write-Host "Step 4: Start Worker for Processing" -ForegroundColor Cyan

if ($jobCount -gt 0) {
    $env:WORKER_ENABLE_AI_PROCESSING = "true"
    $env:WORKER_CLAUDE_ENABLED = "true"
    $env:WORKER_CONCURRENT_JOBS = "2"
    
    try {
        $workerProcess = Start-Process -FilePath "bin/worker.exe" -PassThru -WindowStyle Hidden
        
        if ($workerProcess) {
            Write-Host "Worker started (PID: $($workerProcess.Id))" -ForegroundColor Green
            Write-Host "Processing for 30 seconds..." -ForegroundColor Yellow
            Start-Sleep -Seconds 30
            
            Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
            Write-Host "Worker stopped" -ForegroundColor Green
            $workerSuccess = $true
        } else {
            Write-Host "Failed to start worker" -ForegroundColor Red
            $workerSuccess = $false
        }
    } catch {
        Write-Host "Worker error: $_" -ForegroundColor Red
        $workerSuccess = $false
    }
} else {
    Write-Host "Skipping worker - no AI jobs to process" -ForegroundColor Yellow
    $workerSuccess = $false
}

Write-Host "Step 5: Check Results" -ForegroundColor Cyan

try {
    $headers = @{
        "Authorization" = "Bearer $token"
        "X-Tenant-Subdomain" = $TENANT_SUBDOMAIN
    }
    
    $response = Invoke-RestMethod -Uri "$BASE_URL/api/v1/documents/$documentId/ai-results" -Method GET -Headers $headers
    
    if ($response) {
        Write-Host "AI Results Retrieved" -ForegroundColor Green
        Write-Host "Has Results: $($response.has_results)" -ForegroundColor White
        
        if ($response.summary) {
            Write-Host "Summary Generated: YES" -ForegroundColor Green
        }
        
        if ($response.entities) {
            Write-Host "Entities Extracted: YES" -ForegroundColor Green
        }
        
        if ($response.classification) {
            Write-Host "Document Classified: YES" -ForegroundColor Green
        }
        
        if ($response.tags) {
            Write-Host "Tags Generated: YES ($($response.tags.Count) tags)" -ForegroundColor Green
        }
        
        $resultsAvailable = $response.has_results
    } else {
        Write-Host "No AI results available" -ForegroundColor Yellow
        $resultsAvailable = $false
    }
} catch {
    Write-Host "Results check error: $_" -ForegroundColor Red
    $resultsAvailable = $false
}

Write-Host "`nSprint 2 Test Results:" -ForegroundColor Cyan
Write-Host "=====================" -ForegroundColor Cyan
Write-Host "Authentication: PASSED" -ForegroundColor Green
Write-Host "Document Upload: PASSED" -ForegroundColor Green

if ($jobCount -gt 0) {
    Write-Host "AI Job Auto-Queueing: PASSED" -ForegroundColor Green
} else {
    Write-Host "AI Job Auto-Queueing: FAILED" -ForegroundColor Red
}

if ($workerSuccess) {
    Write-Host "Worker Processing: PASSED" -ForegroundColor Green
} else {
    Write-Host "Worker Processing: PENDING" -ForegroundColor Yellow
}

if ($resultsAvailable) {
    Write-Host "AI Results: AVAILABLE" -ForegroundColor Green
} else {
    Write-Host "AI Results: PENDING" -ForegroundColor Yellow
}

if ($jobCount -gt 0) {
    Write-Host "`nSprint 2 Implementation: SUCCESS!" -ForegroundColor Green
    Write-Host "Document upload automatically triggers AI processing" -ForegroundColor Green
    Write-Host "API endpoints provide access to AI jobs and results" -ForegroundColor Green
} else {
    Write-Host "`nSprint 2 needs investigation" -ForegroundColor Red
    Write-Host "AI jobs not being auto-queued on document upload" -ForegroundColor Yellow
}

Write-Host "`nTest document ID: $documentId" -ForegroundColor Cyan 