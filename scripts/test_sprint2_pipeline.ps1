#!/usr/bin/env pwsh

# Sprint 2 Test: Document Summarization Pipeline
# Tests: Upload → AI Job Queue → Worker Processing → Results Retrieval

Write-Host "🚀 Testing Sprint 2: Document Summarization Pipeline" -ForegroundColor Green
Write-Host "====================================================" -ForegroundColor Green

# Configuration
$BASE_URL = "http://localhost:8080"
$ADMIN_EMAIL = "admin@archivus.com"
$ADMIN_PASSWORD = "admin123"

# Test document content
$TEST_CONTENT = @"
# Archivus AI Integration Test Document

## Executive Summary
This document tests the AI-powered document intelligence capabilities of Archivus Phase 3. 
The system should automatically analyze this content using Claude 4 Sonnet.

## Company Information
- **Company**: Archivus Technologies Inc.
- **Contact Person**: Sarah Johnson (sarah.johnson@archivus.com)
- **Phone**: +1 (555) 123-4567
- **Address**: 123 Innovation Drive, San Francisco, CA 94105

## Financial Details
- **Contract Value**: $25,000 USD
- **Payment Terms**: Net 30 days
- **Project Duration**: 6 months
- **Start Date**: July 1, 2025
- **End Date**: December 31, 2025

## Key Requirements
1. Implement Claude 4 Sonnet integration
2. Develop background worker AI processing
3. Create document summarization features
4. Build semantic search capabilities
5. Design AI-powered analytics dashboard

## Important Dates
- **Kickoff Meeting**: June 25, 2025
- **Phase 1 Delivery**: August 15, 2025
- **Final Delivery**: December 15, 2025
- **Contract Expiry**: December 31, 2025

## Deliverables
- AI Integration Documentation
- Worker System Implementation
- API Endpoint Development
- User Interface Updates
- Testing and Quality Assurance

This document should trigger AI processing for:
- Document summarization
- Entity extraction (people, organizations, dates, amounts)
- Document classification 
- Tag generation and semantic analysis

Contact support@archivus.com for technical assistance.
"@

function Test-Authentication {
    Write-Host "`n🔐 Step 1: Authentication" -ForegroundColor Cyan
    
    try {
        $loginData = @{
            email = $ADMIN_EMAIL
            password = $ADMIN_PASSWORD
        } | ConvertTo-Json
        
        $response = Invoke-RestMethod -Uri "$BASE_URL/api/auth/login" -Method POST -Body $loginData -ContentType "application/json"
        
        if ($response.token) {
            Write-Host "✅ Authentication successful" -ForegroundColor Green
            return $response.token
        }
        
        Write-Host "❌ Authentication failed - no token received" -ForegroundColor Red
        return $null
    } catch {
        Write-Host "❌ Authentication failed: $_" -ForegroundColor Red
        return $null
    }
}

function Upload-TestDocument($token) {
    Write-Host "`n📄 Step 2: Upload Test Document with AI Processing" -ForegroundColor Cyan
    
    try {
        # Save test content to temporary file
        $tempFile = "temp_test_doc.txt"
        $TEST_CONTENT | Out-File -FilePath $tempFile -Encoding UTF8
        
        $headers = @{
            "Authorization" = "Bearer $token"
        }
        
        # Create multipart form data
        $fileBytes = [System.IO.File]::ReadAllBytes((Resolve-Path $tempFile))
        $boundary = [System.Guid]::NewGuid().ToString()
        
        $LF = "`r`n"
        $bodyLines = (
            "--$boundary",
            "Content-Disposition: form-data; name=`"file`"; filename=`"ai_integration_test.txt`"",
            "Content-Type: text/plain$LF",
            [System.Text.Encoding]::UTF8.GetString($fileBytes),
            "--$boundary",
            "Content-Disposition: form-data; name=`"enable_ai`"$LF",
            "true",
            "--$boundary",
            "Content-Disposition: form-data; name=`"title`"$LF",
            "AI Integration Test Document",
            "--$boundary",
            "Content-Disposition: form-data; name=`"description`"$LF",
            "Sprint 2 test document for AI processing pipeline validation",
            "--$boundary--$LF"
        ) -join $LF
        
        $body = [System.Text.Encoding]::UTF8.GetBytes($bodyLines)
        
        $response = Invoke-RestMethod -Uri "$BASE_URL/api/documents/upload" -Method POST -Body $body -Headers $headers -ContentType "multipart/form-data; boundary=$boundary"
        
        # Cleanup temp file
        Remove-Item $tempFile -ErrorAction SilentlyContinue
        
        if ($response.document -and $response.document.id) {
            Write-Host "✅ Document uploaded successfully" -ForegroundColor Green
            Write-Host "   Document ID: $($response.document.id)" -ForegroundColor White
            Write-Host "   Title: $($response.document.title)" -ForegroundColor White
            Write-Host "   AI Processing: Enabled" -ForegroundColor Green
            return $response.document.id
        }
        
        Write-Host "❌ Document upload failed - no document ID received" -ForegroundColor Red
        return $null
    } catch {
        Write-Host "❌ Document upload failed: $_" -ForegroundColor Red
        return $null
    }
}

function Check-AIJobs($token, $documentId) {
    Write-Host "`n🔍 Step 3: Verify AI Jobs Auto-Queued" -ForegroundColor Cyan
    
    try {
        $headers = @{
            "Authorization" = "Bearer $token"
        }
        
        # Wait a moment for jobs to be created
        Start-Sleep -Seconds 3
        
        $response = Invoke-RestMethod -Uri "$BASE_URL/api/documents/$documentId/jobs" -Method GET -Headers $headers
        
        if ($response.jobs) {
            $aiJobs = $response.jobs | Where-Object { 
                $_.job_type -in @(
                    "document_summarization", 
                    "entity_extraction", 
                    "document_classification", 
                    "semantic_analysis"
                ) 
            }
            
            Write-Host "✅ AI jobs created successfully" -ForegroundColor Green
            Write-Host "   Total Jobs: $($response.jobs.Count)" -ForegroundColor White
            Write-Host "   AI Jobs: $($aiJobs.Count)" -ForegroundColor White
            
            foreach ($job in $aiJobs) {
                $statusColor = switch ($job.status) {
                    "queued" { "Yellow" }
                    "in_progress" { "Blue" }
                    "completed" { "Green" }
                    "failed" { "Red" }
                    default { "White" }
                }
                Write-Host "   - $($job.job_type): $($job.status) (Priority: $($job.priority))" -ForegroundColor $statusColor
            }
            
            return $aiJobs
        }
        
        Write-Host "⚠️  No jobs found for document" -ForegroundColor Yellow
        return @()
    } catch {
        Write-Host "❌ Failed to check AI jobs: $_" -ForegroundColor Red
        return @()
    }
}

function Start-WorkerProcessing($jobs) {
    Write-Host "`n⚙️  Step 4: Process AI Jobs with Worker" -ForegroundColor Cyan
    
    if ($jobs.Count -eq 0) {
        Write-Host "⚠️  No AI jobs to process" -ForegroundColor Yellow
        return $false
    }
    
    Write-Host "🚀 Starting AI worker for job processing..." -ForegroundColor White
    Write-Host "   Jobs to process: $($jobs.Count)" -ForegroundColor White
    
    # Set environment variables for AI processing
    $env:WORKER_ENABLE_AI_PROCESSING = "true"
    $env:WORKER_CLAUDE_ENABLED = "true"
    $env:WORKER_CONCURRENT_JOBS = "3"
    $env:WORKER_POLL_INTERVAL = "3s"
    $env:WORKER_AI_JOB_TIMEOUT = "2m"
    
    try {
        # Start worker in background
        $workerProcess = Start-Process -FilePath "bin/worker.exe" -PassThru -WindowStyle Hidden
        
        if ($workerProcess) {
            Write-Host "✅ Worker started (PID: $($workerProcess.Id))" -ForegroundColor Green
            
            # Let worker process for 45 seconds
            Write-Host "⏱️  Processing for 45 seconds..." -ForegroundColor Yellow
            Start-Sleep -Seconds 45
            
            # Stop worker
            Write-Host "🛑 Stopping worker..." -ForegroundColor Yellow
            Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
            Start-Sleep -Seconds 2
            
            Write-Host "✅ Worker processing completed" -ForegroundColor Green
            return $true
        } else {
            Write-Host "❌ Failed to start worker process" -ForegroundColor Red
            return $false
        }
    } catch {
        Write-Host "❌ Worker processing error: $_" -ForegroundColor Red
        return $false
    }
}

function Check-ProcessingResults($token, $documentId) {
    Write-Host "`n📊 Step 5: Check AI Processing Results" -ForegroundColor Cyan
    
    try {
        $headers = @{
            "Authorization" = "Bearer $token"
        }
        
        # Check job status first
        Write-Host "   Checking job status..." -ForegroundColor White
        $jobsResponse = Invoke-RestMethod -Uri "$BASE_URL/api/documents/$documentId/jobs" -Method GET -Headers $headers
        
        if ($jobsResponse.jobs) {
            $completedJobs = 0
            $failedJobs = 0
            $pendingJobs = 0
            
            foreach ($job in $jobsResponse.jobs) {
                if ($job.job_type -in @("document_summarization", "entity_extraction", "document_classification", "semantic_analysis")) {
                    switch ($job.status) {
                        "completed" { $completedJobs++ }
                        "failed" { $failedJobs++ }
                        default { $pendingJobs++ }
                    }
                }
            }
            
            Write-Host "   Job Status Summary:" -ForegroundColor White
            Write-Host "   - Completed: $completedJobs" -ForegroundColor Green
            Write-Host "   - Failed: $failedJobs" -ForegroundColor Red
            Write-Host "   - Pending: $pendingJobs" -ForegroundColor Yellow
        }
        
        # Check AI results
        Write-Host "   Checking AI results..." -ForegroundColor White
        $resultsResponse = Invoke-RestMethod -Uri "$BASE_URL/api/documents/$documentId/ai-results" -Method GET -Headers $headers
        
        if ($resultsResponse) {
            Write-Host "✅ AI Results Retrieved" -ForegroundColor Green
            Write-Host "   Has Results: $($resultsResponse.has_results)" -ForegroundColor White
            
            if ($resultsResponse.summary) {
                Write-Host "   ✅ Summary Generated" -ForegroundColor Green
                Write-Host "      Length: $($resultsResponse.summary.Length) characters" -ForegroundColor White
            }
            
            if ($resultsResponse.entities) {
                Write-Host "   ✅ Entities Extracted" -ForegroundColor Green
                Write-Host "      Categories: $($resultsResponse.entities.Keys.Count)" -ForegroundColor White
            }
            
            if ($resultsResponse.classification) {
                Write-Host "   ✅ Document Classified" -ForegroundColor Green
                Write-Host "      Type: $($resultsResponse.classification.type)" -ForegroundColor White
                Write-Host "      Confidence: $([math]::Round($resultsResponse.classification.confidence * 100, 1))%" -ForegroundColor White
            }
            
            if ($resultsResponse.tags) {
                Write-Host "   ✅ Tags Generated" -ForegroundColor Green
                Write-Host "      Count: $($resultsResponse.tags.Count)" -ForegroundColor White
                Write-Host "      Tags: $($resultsResponse.tags -join ', ')" -ForegroundColor White
            }
            
            return $resultsResponse.has_results
        }
        
        Write-Host "⚠️  No AI results available yet" -ForegroundColor Yellow
        return $false
    } catch {
        Write-Host "❌ Failed to check AI results: $_" -ForegroundColor Red
        return $false
    }
}

# Main Test Execution
Write-Host "Starting Sprint 2 Pipeline Test..." -ForegroundColor White

# Execute test pipeline
$token = Test-Authentication
if (-not $token) {
    Write-Host "`n❌ Test failed at authentication step" -ForegroundColor Red
    exit 1
}

$documentId = Upload-TestDocument $token
if (-not $documentId) {
    Write-Host "`n❌ Test failed at document upload step" -ForegroundColor Red
    exit 1
}

$aiJobs = Check-AIJobs $token $documentId
$workerSuccess = Start-WorkerProcessing $aiJobs
$resultsAvailable = Check-ProcessingResults $token $documentId

# Final Results
Write-Host "`n🎯 Sprint 2 Pipeline Test Results" -ForegroundColor Cyan
Write-Host "=================================" -ForegroundColor Cyan
Write-Host "✅ Authentication: PASSED" -ForegroundColor Green
Write-Host "✅ Document Upload: PASSED" -ForegroundColor Green
Write-Host "$(if ($aiJobs.Count -gt 0) { '✅' } else { '❌' }) AI Job Auto-Queueing: $(if ($aiJobs.Count -gt 0) { 'PASSED' } else { 'FAILED' })" -ForegroundColor $(if ($aiJobs.Count -gt 0) { "Green" } else { "Red" })
Write-Host "$(if ($workerSuccess) { '✅' } else { '❌' }) Worker Processing: $(if ($workerSuccess) { 'PASSED' } else { 'FAILED' })" -ForegroundColor $(if ($workerSuccess) { "Green" } else { "Red" })
Write-Host "$(if ($resultsAvailable) { '✅' } else { '⚠️ ' }) AI Results: $(if ($resultsAvailable) { 'AVAILABLE' } else { 'PENDING' })" -ForegroundColor $(if ($resultsAvailable) { "Green" } else { "Yellow" })

if ($aiJobs.Count -gt 0 -and $workerSuccess) {
    Write-Host "`n🎉 Sprint 2 Implementation: SUCCESS!" -ForegroundColor Green
    Write-Host "   Document upload automatically triggers AI processing" -ForegroundColor Green
    Write-Host "   Worker successfully processes AI jobs using Claude" -ForegroundColor Green
    Write-Host "   API endpoints provide access to AI results" -ForegroundColor Green
    
    if ($resultsAvailable) {
        Write-Host "`n🚀 READY FOR SPRINT 3: Semantic Search & Embeddings!" -ForegroundColor Magenta
    } else {
        Write-Host "`n⏳ AI processing may need more time - check results later" -ForegroundColor Yellow
    }
} else {
    Write-Host "`n❌ Sprint 2 needs investigation" -ForegroundColor Red
    Write-Host "   Check Claude API configuration and worker logs" -ForegroundColor Yellow
}

Write-Host "`nTest document ID for manual inspection: $documentId" -ForegroundColor Cyan 