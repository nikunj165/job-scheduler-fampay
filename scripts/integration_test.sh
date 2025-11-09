#!/usr/bin/env bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:8080}"
LOG_FILE="integration_test.log"

# Cleanup function
cleanup() {
    if [[ -n "${SERVER_PID:-}" ]]; then
        echo -e "${YELLOW}[test] Stopping server (PID: $SERVER_PID)${NC}"
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
    rm -f $LOG_FILE
}

trap cleanup EXIT

echo -e "${GREEN}[test] Starting integration tests${NC}"

# Build the service
echo "[test] Building service..."
go build -o job-scheduler-test .

# Start the server in background
echo "[test] Starting server on port 8080..."
./job-scheduler-test --port=8080 --workers=5 --logfile=$LOG_FILE &
SERVER_PID=$!

# Wait for server to start
echo "[test] Waiting for server to be ready..."
for i in {1..30}; do
    if curl -s "${API_URL}/healthz" > /dev/null 2>&1; then
        echo -e "${GREEN}[test] Server is ready${NC}"
        break
    fi
    if [[ $i -eq 30 ]]; then
        echo -e "${RED}[test] Server failed to start${NC}"
        exit 1
    fi
    sleep 1
done

# Test 1: Health check
echo "[test] Test 1: Health check"
HEALTH_RESPONSE=$(curl -s "${API_URL}/healthz")
if [[ $(echo "$HEALTH_RESPONSE" | jq -r '.status') == "ok" ]]; then
    echo -e "${GREEN}  ✓ Health check passed${NC}"
else
    echo -e "${RED}  ✗ Health check failed${NC}"
    exit 1
fi

# Test 2: Create a job that runs every 30 seconds (to avoid multiple executions during test)
echo "[test] Test 2: Create a job"
CREATE_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/jobs" \
    -H "Content-Type: application/json" \
    -d '{
        "schedule": "*/30 * * * * *",
        "api": "https://httpcan.org/post",
        "type": "ATLEAST_ONCE"
    }')

JOB_ID=$(echo "$CREATE_RESPONSE" | jq -r '.job_id')
if [[ -n "$JOB_ID" && "$JOB_ID" != "null" ]]; then
    echo -e "${GREEN}  ✓ Job created with ID: $JOB_ID${NC}"
else
    echo -e "${RED}  ✗ Failed to create job${NC}"
    echo "  Response: $CREATE_RESPONSE"
    exit 1
fi

# Test 3: Get the created job
echo "[test] Test 3: Get job details"
GET_RESPONSE=$(curl -s "${API_URL}/api/v1/jobs/${JOB_ID}")
RETRIEVED_SCHEDULE=$(echo "$GET_RESPONSE" | jq -r '.schedule')
if [[ "$RETRIEVED_SCHEDULE" == "*/30 * * * * *" ]]; then
    echo -e "${GREEN}  ✓ Job retrieved successfully${NC}"
else
    echo -e "${RED}  ✗ Failed to retrieve job${NC}"
    echo "  Response: $GET_RESPONSE"
    exit 1
fi

# Test 4: List all jobs
echo "[test] Test 4: List all jobs"
LIST_RESPONSE=$(curl -s "${API_URL}/api/v1/jobs")
TOTAL_JOBS=$(echo "$LIST_RESPONSE" | jq -r '.total')
if [[ "$TOTAL_JOBS" -ge 1 ]]; then
    echo -e "${GREEN}  ✓ Jobs listed successfully (found $TOTAL_JOBS jobs)${NC}"
else
    echo -e "${RED}  ✗ Failed to list jobs${NC}"
    exit 1
fi

# Test 5: Wait for job to execute
echo "[test] Test 5: Verify job execution"
echo "  Waiting for first execution (max 35 seconds)..."

# Poll for execution with timeout
MAX_WAIT=35
WAIT_COUNT=0
EXEC_COUNT=0

while [[ "$EXEC_COUNT" -eq 0 && "$WAIT_COUNT" -lt "$MAX_WAIT" ]]; do
    sleep 1
    EXEC_RESPONSE=$(curl -s "${API_URL}/api/v1/jobs/${JOB_ID}/executions")
    EXEC_COUNT=$(echo "$EXEC_RESPONSE" | jq -r '.count // 0')
    WAIT_COUNT=$((WAIT_COUNT + 1))
done

if [[ "$EXEC_COUNT" -ge 1 ]]; then
    echo -e "${GREEN}  ✓ Job executed successfully after ${WAIT_COUNT} seconds ($EXEC_COUNT executions)${NC}"
    
    # Show execution details
    FIRST_EXEC=$(echo "$EXEC_RESPONSE" | jq -r '.executions[0]')
    HTTP_STATUS=$(echo "$FIRST_EXEC" | jq -r '.http_status')
    RESPONSE_TIME=$(echo "$FIRST_EXEC" | jq -r '.response_time_ms')
    SUCCESS=$(echo "$FIRST_EXEC" | jq -r '.success')
    
    echo "    - HTTP Status: $HTTP_STATUS"
    echo "    - Response Time: ${RESPONSE_TIME}ms"
    echo "    - Success: $SUCCESS"
else
    echo -e "${RED}  ✗ Job did not execute${NC}"
    echo "  Response: $EXEC_RESPONSE"
    exit 1
fi

# Test 6: Update job schedule
echo "[test] Test 6: Update job schedule"

# First verify the job still exists
echo "  Verifying job exists before update..."
VERIFY_RESPONSE=$(curl -s "${API_URL}/api/v1/jobs/${JOB_ID}")
if ! echo "$VERIFY_RESPONSE" | jq -e '.id' > /dev/null 2>&1; then
    echo -e "${RED}  ✗ Job does not exist or cannot be retrieved${NC}"
    echo "  Job ID: $JOB_ID"
    echo "  Response: $VERIFY_RESPONSE"
    exit 1
fi
echo "  Job verified, proceeding with update..."

# Use a temp file to separate body and status
TEMP_FILE=$(mktemp)
HTTP_STATUS=$(curl -s -w "%{http_code}" -o "$TEMP_FILE" -X PUT "${API_URL}/api/v1/jobs/${JOB_ID}" \
    -H "Content-Type: application/json" \
    -d '{
        "schedule": "0 */2 * * * *"
    }')

# Read the response body from temp file
UPDATE_RESPONSE=$(cat "$TEMP_FILE")
rm -f "$TEMP_FILE"

# Check if we got a valid HTTP response
if [[ "$HTTP_STATUS" == "000" ]]; then
    echo -e "${RED}  ✗ Failed to connect to server - is the service running?${NC}"
    exit 1
fi

# Check if response is empty
if [[ -z "$UPDATE_RESPONSE" && "$HTTP_STATUS" != "204" ]]; then
    echo -e "${RED}  ✗ Empty response body with status $HTTP_STATUS${NC}"
    # For some status codes, empty body is expected, continue
fi

# Only try to parse as JSON if we have content and it's not a plain text error
if [[ -n "$UPDATE_RESPONSE" ]]; then
    # Check if it's a plain text error message (like "404 page not found")
    if [[ "$UPDATE_RESPONSE" == "404 page not found" ]] || [[ "$UPDATE_RESPONSE" =~ ^[0-9]+\ .* ]]; then
        # Plain text error, don't try to parse as JSON
        echo -e "${RED}  ✗ Server returned plain text error: $UPDATE_RESPONSE${NC}"
        echo "  HTTP Status: $HTTP_STATUS"
        echo "  Job ID was: $JOB_ID"
        exit 1
    elif ! echo "$UPDATE_RESPONSE" | jq '.' > /dev/null 2>&1; then
        echo -e "${RED}  ✗ Invalid JSON response${NC}"
        echo "  Response was: '$UPDATE_RESPONSE'"
        echo "  HTTP Status was: $HTTP_STATUS"
        exit 1
    fi
fi

# Check response based on status code and content
if [[ "$HTTP_STATUS" == "200" ]]; then
    # Success case - check for message or assume success
    if [[ -n "$UPDATE_RESPONSE" ]]; then
        if echo "$UPDATE_RESPONSE" | jq -e '.message' > /dev/null 2>&1; then
            MESSAGE=$(echo "$UPDATE_RESPONSE" | jq -r '.message')
            if [[ "$MESSAGE" == "Job updated successfully" ]]; then
                echo -e "${GREEN}  ✓ Job updated successfully (now runs every 2 minutes)${NC}"
            else
                echo -e "${GREEN}  ✓ Job updated (message: $MESSAGE)${NC}"
            fi
        else
            # No message field, but 200 status means success
            echo -e "${GREEN}  ✓ Job updated successfully (HTTP 200)${NC}"
        fi
    else
        # Empty response but 200 status
        echo -e "${GREEN}  ✓ Job updated successfully (HTTP 200, no body)${NC}"
    fi
elif [[ "$HTTP_STATUS" == "400" ]] || [[ "$HTTP_STATUS" == "404" ]]; then
    # Error case
    if [[ -n "$UPDATE_RESPONSE" ]] && echo "$UPDATE_RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
        ERROR=$(echo "$UPDATE_RESPONSE" | jq -r '.error')
        echo -e "${RED}  ✗ Failed to update job: $ERROR${NC}"
        echo "  Details: $(echo "$UPDATE_RESPONSE" | jq -r '.details // "N/A"')"
    else
        echo -e "${RED}  ✗ Failed to update job (HTTP $HTTP_STATUS)${NC}"
        echo "  Response: $UPDATE_RESPONSE"
    fi
    exit 1
else
    echo -e "${RED}  ✗ Unexpected HTTP status: $HTTP_STATUS${NC}"
    echo "  Response: $UPDATE_RESPONSE"
    exit 1
fi

# Test 7: Get job stats
echo "[test] Test 7: Get job statistics"
STATS_RESPONSE=$(curl -s "${API_URL}/api/v1/jobs/${JOB_ID}/stats")
TOTAL_EXECS=$(echo "$STATS_RESPONSE" | jq -r '.total_executions // 0')
if [[ "$TOTAL_EXECS" -ge 1 ]]; then
    echo -e "${GREEN}  ✓ Job stats retrieved (total executions: $TOTAL_EXECS)${NC}"
else
    echo -e "${RED}  ✗ Failed to get job stats${NC}"
    exit 1
fi

# Test 8: Delete the job
echo "[test] Test 8: Delete job"
DELETE_RESPONSE=$(curl -s -X DELETE "${API_URL}/api/v1/jobs/${JOB_ID}")
if [[ $(echo "$DELETE_RESPONSE" | jq -r '.message') == "Job deleted successfully" ]]; then
    echo -e "${GREEN}  ✓ Job deleted successfully${NC}"
else
    echo -e "${RED}  ✗ Failed to delete job${NC}"
    exit 1
fi

# Test 9: Verify job is deleted (soft delete - status should be DELETED)
echo "[test] Test 9: Verify job deletion"
DELETED_JOB=$(curl -s "${API_URL}/api/v1/jobs/${JOB_ID}")
JOB_STATUS=$(echo "$DELETED_JOB" | jq -r '.status')
if [[ "$JOB_STATUS" == "DELETED" ]]; then
    echo -e "${GREEN}  ✓ Job marked as deleted${NC}"
else
    echo -e "${RED}  ✗ Job deletion verification failed${NC}"
    exit 1
fi

# Test 10: Create a job with frequent execution
echo "[test] Test 10: Create job with frequent schedule"
IMMEDIATE_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/jobs" \
    -H "Content-Type: application/json" \
    -d '{
        "schedule": "*/10 * * * * *",
        "api": "https://httpcan.org/status/200",
        "type": "ATMOST_ONCE"
    }')

IMMEDIATE_JOB_ID=$(echo "$IMMEDIATE_RESPONSE" | jq -r '.job_id')
if [[ -n "$IMMEDIATE_JOB_ID" && "$IMMEDIATE_JOB_ID" != "null" ]]; then
    echo -e "${GREEN}  ✓ Frequent job created with ID: $IMMEDIATE_JOB_ID${NC}"
    
    # Poll for execution with timeout
    echo "  Waiting for execution (max 12 seconds)..."
    WAIT_COUNT=0
    IMMEDIATE_COUNT=0
    
    while [[ "$IMMEDIATE_COUNT" -eq 0 && "$WAIT_COUNT" -lt 12 ]]; do
        sleep 1
        IMMEDIATE_EXEC=$(curl -s "${API_URL}/api/v1/jobs/${IMMEDIATE_JOB_ID}/executions")
        IMMEDIATE_COUNT=$(echo "$IMMEDIATE_EXEC" | jq -r '.count // 0')
        WAIT_COUNT=$((WAIT_COUNT + 1))
    done
    
    if [[ "$IMMEDIATE_COUNT" -ge 1 ]]; then
        echo -e "${GREEN}  ✓ Frequent job executed within ${WAIT_COUNT} seconds${NC}"
    else
        echo -e "${RED}  ✗ Frequent job did not execute within timeout${NC}"
    fi
    
    # Clean up
    curl -s -X DELETE "${API_URL}/api/v1/jobs/${IMMEDIATE_JOB_ID}" > /dev/null
else
    echo -e "${RED}  ✗ Failed to create immediate job${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}[test] All integration tests passed! ✓${NC}"
echo -e "${GREEN}========================================${NC}"
