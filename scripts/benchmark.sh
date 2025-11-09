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
WARMUP_JOBS="${WARMUP_JOBS:-100}"
BENCHMARK_JOBS="${BENCHMARK_JOBS:-1000}"
BATCH_SIZE="${BATCH_SIZE:-50}"

echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}                 JOB SCHEDULER PERFORMANCE BENCHMARK            ${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo ""

# Check if service is running
echo "[check] Verifying service availability..."
if ! curl -s "${API_URL}/healthz" > /dev/null 2>&1; then
    echo -e "${RED}[error] Service is not running at ${API_URL}${NC}"
    echo "Please start the service with: go run . --optimized"
    exit 1
fi
echo -e "${GREEN}[check] Service is running${NC}"
echo ""

# Function to create jobs in batch
create_job_batch() {
    local count=$1
    local prefix=$2
    local created=0
    local failed=0
    
    for i in $(seq 1 $count); do
        # Rotate through different schedules and endpoints
        local schedule_options=("*/5 * * * * *" "*/10 * * * * *" "*/30 * * * * *" "0 * * * * *")
        local endpoint_options=(
            "https://httpcan.org/status/200"
            "https://httpcan.org/delay/0"
            "https://httpcan.org/json"
        )
        
        local schedule="${schedule_options[$((i % ${#schedule_options[@]}))]}"
        local endpoint="${endpoint_options[$((i % ${#endpoint_options[@]}))]}"
        
        local response=$(curl -s -w "\nHTTP:%{http_code}" -X POST "${API_URL}/api/v1/jobs" \
            -H "Content-Type: application/json" \
            -d "{
                \"schedule\": \"$schedule\",
                \"api\": \"$endpoint\",
                \"type\": \"ATLEAST_ONCE\"
            }" 2>/dev/null)
        
        local http_status=$(echo "$response" | tail -n1 | cut -d: -f2)
        
        if [[ "$http_status" == "200" ]] || [[ "$http_status" == "201" ]]; then
            created=$((created + 1))
        else
            failed=$((failed + 1))
        fi
        
        # Progress indicator
        if [[ $((i % 10)) -eq 0 ]]; then
            echo -n "."
        fi
    done
    
    echo ""
    echo "$created|$failed"
}

# Warmup phase
echo -e "${YELLOW}[warmup] Creating $WARMUP_JOBS jobs to warm up the system...${NC}"
warmup_start=$(date +%s%N)
IFS='|' read -r warmup_created warmup_failed <<< "$(create_job_batch $WARMUP_JOBS "warmup")"
warmup_end=$(date +%s%N)
warmup_duration=$(( (warmup_end - warmup_start) / 1000000 )) # ms
warmup_rate=$(( (warmup_created * 1000) / warmup_duration )) # jobs/sec

echo "[warmup] Created: $warmup_created, Failed: $warmup_failed"
echo "[warmup] Duration: ${warmup_duration}ms, Rate: $warmup_rate jobs/sec"
echo ""

# Main benchmark
echo -e "${GREEN}[benchmark] Creating $BENCHMARK_JOBS jobs in batches of $BATCH_SIZE...${NC}"

total_created=0
total_failed=0
benchmark_start=$(date +%s%N)

# Create jobs in batches
batches=$((BENCHMARK_JOBS / BATCH_SIZE))
for batch in $(seq 1 $batches); do
    echo "[batch $batch/$batches] Creating $BATCH_SIZE jobs..."
    
    batch_start=$(date +%s%N)
    IFS='|' read -r created failed <<< "$(create_job_batch $BATCH_SIZE "batch$batch")"
    batch_end=$(date +%s%N)
    
    total_created=$((total_created + created))
    total_failed=$((total_failed + failed))
    
    batch_duration=$(( (batch_end - batch_start) / 1000000 )) # ms
    batch_rate=$(( (created * 1000) / batch_duration )) # jobs/sec
    
    echo "[batch $batch] Created: $created, Failed: $failed, Rate: $batch_rate jobs/sec"
done

benchmark_end=$(date +%s%N)
benchmark_duration=$(( (benchmark_end - benchmark_start) / 1000000000 )) # seconds
overall_rate=$((total_created / benchmark_duration))

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}                        BENCHMARK RESULTS                       ${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo ""

echo -e "${GREEN}Summary:${NC}"
echo "  • Total Jobs Created: $total_created"
echo "  • Total Failed: $total_failed"
echo "  • Test Duration: ${benchmark_duration} seconds"
echo "  • Average Rate: $overall_rate jobs/sec"
echo ""

echo -e "${GREEN}Performance Analysis:${NC}"
if [[ $overall_rate -ge 1000 ]]; then
    echo -e "  ${GREEN}✓ EXCELLENT: Achieved ${overall_rate} jobs/sec (>1000)${NC}"
elif [[ $overall_rate -ge 800 ]]; then
    echo -e "  ${GREEN}✓ VERY GOOD: Achieved ${overall_rate} jobs/sec${NC}"
elif [[ $overall_rate -ge 500 ]]; then
    echo -e "  ${YELLOW}○ GOOD: Achieved ${overall_rate} jobs/sec${NC}"
else
    echo -e "  ${RED}✗ NEEDS OPTIMIZATION: Only ${overall_rate} jobs/sec${NC}"
fi

# Check system stats
echo ""
echo -e "${GREEN}System Statistics:${NC}"
stats_response=$(curl -s "${API_URL}/api/v1/stats" 2>/dev/null || echo "{}")
if [[ -n "$stats_response" ]]; then
    active_jobs=$(echo "$stats_response" | jq -r '.active_jobs // 0' 2>/dev/null || echo 0)
    total_jobs=$(echo "$stats_response" | jq -r '.total_jobs // 0' 2>/dev/null || echo 0)
    echo "  • Active Jobs: $active_jobs"
    echo "  • Total Jobs: $total_jobs"
fi

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"

# Exit with appropriate code
if [[ $overall_rate -ge 500 ]]; then
    exit 0
else
    exit 1
fi
