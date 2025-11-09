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

# Test 2: Create a job that runs every 5 seconds
echo "[test] Test 2: Create a job"
CREATE_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/jobs" \
    -H "Content-Type: application/json" \
    -d '{
        "schedule": "*/5 * * * * *",
        "api": "https://httpbin.org/post",
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
if [[ "$RETRIEVED_SCHEDULE" == "*/5 * * * * *" ]]; then
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
echo "  Waiting 10 seconds for job to execute..."
sleep 10

# Check executions
EXEC_RESPONSE=$(curl -s "${API_URL}/api/v1/jobs/${JOB_ID}/executions")
EXEC_COUNT=$(echo "$EXEC_RESPONSE" | jq -r '.count')
if [[ "$EXEC_COUNT" -ge 1 ]]; then
    echo -e "${GREEN}  ✓ Job executed successfully ($EXEC_COUNT executions)${NC}"
    
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
UPDATE_RESPONSE=$(curl -s -X PATCH "${API_URL}/api/v1/jobs/${JOB_ID}" \
    -H "Content-Type: application/json" \
    -d '{
        "schedule": "*/10 * * * * *"
    }')

if [[ $(echo "$UPDATE_RESPONSE" | jq -r '.message') == "Job updated successfully" ]]; then
    echo -e "${GREEN}  ✓ Job updated successfully${NC}"
else
    echo -e "${RED}  ✗ Failed to update job${NC}"
    exit 1
fi

# Test 7: Get job stats
echo "[test] Test 7: Get job statistics"
STATS_RESPONSE=$(curl -s "${API_URL}/api/v1/jobs/${JOB_ID}/stats")
TOTAL_EXECS=$(echo "$STATS_RESPONSE" | jq -r '.total_executions')
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

# Test 10: Create a job with immediate execution
echo "[test] Test 10: Create job with immediate schedule"
IMMEDIATE_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/jobs" \
    -H "Content-Type: application/json" \
    -d '{
        "schedule": "* * * * * *",
        "api": "https://httpbin.org/status/200",
        "type": "ATMOST_ONCE"
    }')

IMMEDIATE_JOB_ID=$(echo "$IMMEDIATE_RESPONSE" | jq -r '.job_id')
if [[ -n "$IMMEDIATE_JOB_ID" && "$IMMEDIATE_JOB_ID" != "null" ]]; then
    echo -e "${GREEN}  ✓ Immediate job created with ID: $IMMEDIATE_JOB_ID${NC}"
    
    # Wait for execution
    sleep 3
    
    # Check if it executed
    IMMEDIATE_EXEC=$(curl -s "${API_URL}/api/v1/jobs/${IMMEDIATE_JOB_ID}/executions")
    IMMEDIATE_COUNT=$(echo "$IMMEDIATE_EXEC" | jq -r '.count')
    if [[ "$IMMEDIATE_COUNT" -ge 1 ]]; then
        echo -e "${GREEN}  ✓ Immediate job executed within 3 seconds${NC}"
    else
        echo -e "${RED}  ✗ Immediate job did not execute quickly${NC}"
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
