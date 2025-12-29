#!/usr/bin/env bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8888}"
PERF_REQUESTS="${PERF_REQUESTS:-1000}"
PERF_CONCURRENCY="${PERF_CONCURRENCY:-50}"
P95_THRESHOLD="${P95_THRESHOLD:-50}"  # 50ms threshold
OUTPUT_FILE="${OUTPUT_FILE:-perf-results.json}"

log() { echo -e "${BLUE}[PERF]${NC} $1"; }
success() { echo -e "${GREEN}[PERF OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[PERF WARN]${NC} $1"; }
error() { echo -e "${RED}[PERF ERROR]${NC} $1" >&2; }

# Install hey if not present
install_hey() {
    if ! command -v hey &> /dev/null; then
        log "Installing hey load testing tool..."
        go install github.com/rakyll/hey@latest
        success "hey installed"
    fi
}

# Create a test URL and return the code
create_test_url() {
    local response
    response=$(curl -s -X POST "$BASE_URL/shorten" \
        -H "Content-Type: application/json" \
        -d '{"url":"https://example.com/perf-test","strategy":"token"}')
    echo "$response" | jq -r '.code'
}

# Run hey and extract metrics, outputting JSON to stdout
run_perf_test() {
    local test_type=$1
    local url=$2
    local method=${3:-GET}
    local body=${4:-}

    log "Running $test_type test: $method $url" >&2
    log "  Requests: $PERF_REQUESTS, Concurrency: $PERF_CONCURRENCY" >&2

    local hey_args=(-n "$PERF_REQUESTS" -c "$PERF_CONCURRENCY")

    if [[ "$method" == "POST" && -n "$body" ]]; then
        hey_args+=(-m POST -H "Content-Type: application/json" -d "$body")
    fi

    # Run hey and capture output
    local output
    output=$(hey "${hey_args[@]}" "$url" 2>&1)

    # Extract p95 latency (hey outputs percentiles in seconds)
    # Format: "  95% in X.XXXX secs"
    local p95_sec
    p95_sec=$(echo "$output" | grep -E "^\s+95%" | awk '{print $3}')
    p95_sec=${p95_sec:-0}

    # Extract p99 latency
    local p99_sec
    p99_sec=$(echo "$output" | grep -E "^\s+99%" | awk '{print $3}')
    p99_sec=${p99_sec:-0}

    # Extract average latency
    # Format: "  Average:      X.XXXX secs"
    local avg_sec
    avg_sec=$(echo "$output" | grep "Average:" | awk '{print $2}')
    avg_sec=${avg_sec:-0}

    # Extract requests per second
    # Format: "  Requests/sec:		XXXX.XX"
    local rps
    rps=$(echo "$output" | grep "Requests/sec:" | awk '{print $2}')
    rps=${rps:-0}

    # Convert to milliseconds (handle empty values)
    local p95_ms p99_ms avg_ms
    if [[ -n "$p95_sec" && "$p95_sec" != "0" ]]; then
        p95_ms=$(printf "%.2f" "$(echo "$p95_sec * 1000" | bc -l)")
    else
        p95_ms="0.00"
    fi

    if [[ -n "$p99_sec" && "$p99_sec" != "0" ]]; then
        p99_ms=$(printf "%.2f" "$(echo "$p99_sec * 1000" | bc -l)")
    else
        p99_ms="0.00"
    fi

    if [[ -n "$avg_sec" && "$avg_sec" != "0" ]]; then
        avg_ms=$(printf "%.2f" "$(echo "$avg_sec * 1000" | bc -l)")
    else
        avg_ms="0.00"
    fi

    log "  Results: p95=${p95_ms}ms, p99=${p99_ms}ms, avg=${avg_ms}ms, rps=${rps}" >&2

    # Output JSON for this test (to stdout only)
    printf '{"test":"%s","p95_ms":%s,"p99_ms":%s,"avg_ms":%s,"rps":%s,"requests":%s,"concurrency":%s}\n' \
        "$test_type" "$p95_ms" "$p99_ms" "$avg_ms" "$rps" "$PERF_REQUESTS" "$PERF_CONCURRENCY"
}

main() {
    echo "" >&2
    echo "==============================================" >&2
    echo "  Performance Test" >&2
    echo "==============================================" >&2
    echo "" >&2

    install_hey

    log "Configuration:" >&2
    log "  BASE_URL: $BASE_URL" >&2
    log "  Requests: $PERF_REQUESTS" >&2
    log "  Concurrency: $PERF_CONCURRENCY" >&2
    log "  p95 Threshold: ${P95_THRESHOLD}ms" >&2
    echo "" >&2

    # Create test URL for read tests
    log "Creating test URL for read tests..." >&2
    local code
    code=$(create_test_url)

    if [[ -z "$code" || "$code" == "null" ]]; then
        error "Failed to create test URL"
        exit 1
    fi

    success "Created test URL with code: $code" >&2
    echo "" >&2

    # Create temp file for results
    local results_tmp
    results_tmp=$(mktemp)

    # Read (redirect) test - append to temp file
    run_perf_test "redirect" "$BASE_URL/$code" >> "$results_tmp"
    echo "" >&2

    # Write (shorten) test - append to temp file
    run_perf_test "shorten" "$BASE_URL/shorten" POST '{"url":"https://example.com/perf","strategy":"token"}' >> "$results_tmp"
    echo "" >&2

    # Combine results into JSON array
    local json_results
    json_results=$(jq -s '.' "$results_tmp")
    rm -f "$results_tmp"

    # Write results to file
    echo "$json_results" > "$OUTPUT_FILE"
    success "Results written to $OUTPUT_FILE" >&2

    # Check threshold
    local max_p95
    max_p95=$(echo "$json_results" | jq '[.[].p95_ms] | max')

    echo "" >&2
    log "Maximum p95: ${max_p95}ms (threshold: ${P95_THRESHOLD}ms)" >&2

    if (( $(echo "$max_p95 > $P95_THRESHOLD" | bc -l) )); then
        error "FAILED: p95 latency (${max_p95}ms) exceeds threshold (${P95_THRESHOLD}ms)"
        exit 1
    fi

    success "PASSED: All tests within threshold" >&2
}

main "$@"
