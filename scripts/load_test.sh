#!/usr/bin/env bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:8080}"
TOTAL_JOBS="${TOTAL_JOBS:-1000}"
BATCH_SIZE="${BATCH_SIZE:-50}"

echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}                    JOB SCHEDULER LOAD TEST                     ${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo ""

# Check if service is running
echo "[check] Verifying service availability..."
if ! curl -s "${API_URL}/healthz" > /dev/null 2>&1; then
    echo -e "${RED}[error] Service is not running at ${API_URL}${NC}"
    echo ""
    echo "Start the service with one of these commands:"
    echo "  Standard:  go run ."
    echo "  Optimized: go run . --optimized"
    exit 1
fi
echo -e "${GREEN}[check] Service is running${NC}"
echo ""

# Test configuration
echo -e "${YELLOW}Test Configuration:${NC}"
echo "  • Total Jobs: $TOTAL_JOBS"
echo "  • Batch Size: $BATCH_SIZE"
echo "  • API Endpoint: https://httpcan.org/post"
echo ""

# Start time
start_time=$(date +%s)
created=0
failed=0

# Create jobs in batches
echo -e "${BLUE}[test] Creating $TOTAL_JOBS jobs...${NC}"

batches=$((TOTAL_JOBS / BATCH_SIZE))
remaining=$((TOTAL_JOBS % BATCH_SIZE))

for batch in $(seq 1 $batches); do
    echo -n "[batch $batch/$batches] Creating $BATCH_SIZE jobs... "
    
    batch_created=0
    batch_failed=0
    
    for i in $(seq 1 $BATCH_SIZE); do
        # Simple rotating schedule
        schedules=("*/5 * * * * *" "*/10 * * * * *" "*/30 * * * * *")
        schedule="${schedules[$((i % ${#schedules[@]}))]}"
        
        # Create job
        response=$(curl -s -w "\nHTTP:%{http_code}" -X POST "${API_URL}/api/v1/jobs" \
            -H "Content-Type: application/json" \
            -d "{
                \"schedule\": \"$schedule\",
                \"api\": \"https://httpcan.org/post\",
                \"type\": \"ATLEAST_ONCE\"
            }" 2>/dev/null) || true
        
        http_status=$(echo "$response" | tail -n1 | cut -d: -f2)
        
        if [[ "$http_status" == "200" ]] || [[ "$http_status" == "201" ]]; then
            batch_created=$((batch_created + 1))
        else
            batch_failed=$((batch_failed + 1))
        fi
    done
    
    created=$((created + batch_created))
    failed=$((failed + batch_failed))
    
    echo "✓ ($batch_created created, $batch_failed failed)"
done

# Handle remaining jobs
if [[ $remaining -gt 0 ]]; then
    echo -n "[final] Creating $remaining jobs... "
    
    for i in $(seq 1 $remaining); do
        response=$(curl -s -w "\nHTTP:%{http_code}" -X POST "${API_URL}/api/v1/jobs" \
            -H "Content-Type: application/json" \
            -d "{
                \"schedule\": \"*/10 * * * * *\",
                \"api\": \"https://httpcan.org/post\",
                \"type\": \"ATLEAST_ONCE\"
            }" 2>/dev/null) || true
        
        http_status=$(echo "$response" | tail -n1 | cut -d: -f2)
        
        if [[ "$http_status" == "200" ]] || [[ "$http_status" == "201" ]]; then
            created=$((created + 1))
        else
            failed=$((failed + 1))
        fi
    done
    
    echo "✓"
fi

# End time and calculate duration
end_time=$(date +%s)
duration=$((end_time - start_time))

# Calculate rate
if [[ $duration -gt 0 ]]; then
    rate=$((created / duration))
else
    rate=$created
fi

# Get system stats
echo ""
echo -e "${YELLOW}[stats] Getting system statistics...${NC}"
stats_response=$(curl -s "${API_URL}/api/v1/stats" 2>/dev/null || echo "{}")
active_jobs=$(echo "$stats_response" | jq -r '.active_jobs // 0' 2>/dev/null || echo 0)
total_jobs=$(echo "$stats_response" | jq -r '.total_jobs // 0' 2>/dev/null || echo 0)

# Display results
echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}                         TEST RESULTS                           ${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo ""

echo -e "${GREEN}Summary:${NC}"
echo "  • Jobs Created: $created"
echo "  • Jobs Failed: $failed"
echo "  • Duration: ${duration} seconds"
echo "  • Rate: $rate jobs/sec"
echo ""

echo -e "${GREEN}System Status:${NC}"
echo "  • Active Jobs: $active_jobs"
echo "  • Total Jobs: $total_jobs"
echo ""

# Performance verdict
echo -e "${GREEN}Performance:${NC}"
if [[ $rate -ge 100 ]]; then
    echo -e "  ${GREEN}✓ EXCELLENT: ${rate} jobs/sec${NC}"
elif [[ $rate -ge 50 ]]; then
    echo -e "  ${YELLOW}○ GOOD: ${rate} jobs/sec${NC}"
elif [[ $rate -ge 20 ]]; then
    echo -e "  ${YELLOW}○ ACCEPTABLE: ${rate} jobs/sec${NC}"
else
    echo -e "  ${RED}✗ SLOW: ${rate} jobs/sec${NC}"
fi

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"

# Exit code based on success
if [[ $failed -eq 0 ]]; then
    exit 0
else
    exit 1
fi