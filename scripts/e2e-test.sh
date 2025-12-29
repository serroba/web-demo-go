#!/usr/bin/env bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8888}"
DATABASE_URL="${DATABASE_URL:-postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable}"
E2E_TIMEOUT="${E2E_TIMEOUT:-30}"  # seconds to wait for async events

log() { echo -e "${BLUE}[E2E]${NC} $1"; }
success() { echo -e "${GREEN}[E2E OK]${NC} $1"; }
error() { echo -e "${RED}[E2E ERROR]${NC} $1" >&2; }

# Test 1: Create a short URL and verify url_created_events
test_url_creation_flow() {
    log "Testing URL creation flow..."

    local test_url
    test_url="https://example.com/e2e-test-$(date +%s)"

    # Create short URL via API
    local response
    response=$(curl -s -X POST "$BASE_URL/shorten" \
        -H "Content-Type: application/json" \
        -H "User-Agent: E2E-Test/1.0" \
        -d "{\"url\":\"$test_url\",\"strategy\":\"token\"}")

    local code
    code=$(echo "$response" | jq -r '.code' 2>/dev/null || echo "")

    if [[ -z "$code" || "$code" == "null" ]]; then
        error "Failed to create short URL. Response: $response"
        return 1
    fi

    log "Created short URL with code: $code"

    # Wait for event to appear in TimescaleDB
    log "Waiting for url_created_events (timeout: ${E2E_TIMEOUT}s)..."
    for _ in $(seq 1 "$E2E_TIMEOUT"); do
        local count
        count=$(psql "$DATABASE_URL" -t -c \
            "SELECT COUNT(*) FROM url_created_events WHERE code = '$code'" 2>/dev/null || echo "0")
        count=$(echo "$count" | tr -d ' ')

        if [[ "$count" -gt 0 ]]; then
            success "url_created_event found for code $code"
            return 0
        fi
        sleep 1
    done

    error "TIMEOUT: url_created_event not found for code $code after ${E2E_TIMEOUT}s"
    return 1
}

# Test 2: Access a short URL and verify url_accessed_events
test_url_access_flow() {
    log "Testing URL access flow..."

    # First create a URL
    local test_url
    test_url="https://example.com/e2e-access-$(date +%s)"
    local response
    response=$(curl -s -X POST "$BASE_URL/shorten" \
        -H "Content-Type: application/json" \
        -d "{\"url\":\"$test_url\",\"strategy\":\"token\"}")

    local code
    code=$(echo "$response" | jq -r '.code' 2>/dev/null || echo "")

    if [[ -z "$code" || "$code" == "null" ]]; then
        error "Failed to create short URL for access test. Response: $response"
        return 1
    fi

    log "Created short URL with code: $code"

    # Wait a moment for creation event to be processed
    sleep 1

    # Access the short URL (follow redirects disabled to capture 301)
    curl -s -o /dev/null "$BASE_URL/$code" \
        -H "User-Agent: E2E-Test-Access/1.0" \
        -H "Referer: https://test-referrer.com"

    log "Accessed short URL: $code"

    # Wait for access event in TimescaleDB
    log "Waiting for url_accessed_events (timeout: ${E2E_TIMEOUT}s)..."
    for _ in $(seq 1 "$E2E_TIMEOUT"); do
        local count
        count=$(psql "$DATABASE_URL" -t -c \
            "SELECT COUNT(*) FROM url_accessed_events WHERE code = '$code'" 2>/dev/null || echo "0")
        count=$(echo "$count" | tr -d ' ')

        if [[ "$count" -gt 0 ]]; then
            success "url_accessed_event found for code $code"
            return 0
        fi
        sleep 1
    done

    error "TIMEOUT: url_accessed_event not found for code $code after ${E2E_TIMEOUT}s"
    return 1
}

main() {
    echo ""
    echo "=============================================="
    echo "  E2E Test - Full Async Flow Validation"
    echo "=============================================="
    echo ""

    log "Configuration:"
    log "  BASE_URL: $BASE_URL"
    log "  DATABASE_URL: ${DATABASE_URL%%@*}@***"
    log "  Timeout: ${E2E_TIMEOUT}s"
    echo ""

    local failures=0

    if ! test_url_creation_flow; then
        ((failures++))
    fi

    echo ""

    if ! test_url_access_flow; then
        ((failures++))
    fi

    echo ""

    if [[ $failures -gt 0 ]]; then
        error "$failures test(s) failed"
        exit 1
    fi

    success "All E2E tests passed!"
}

main "$@"
