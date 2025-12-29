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
DATABASE_URL="${DATABASE_URL:-postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable}"
DURATION="${DURATION:-30s}"
CONCURRENCY="${CONCURRENCY:-50}"
REQUESTS="${REQUESTS:-1000}"
NUM_CLIENTS="${NUM_CLIENTS:-10}"  # Simulated clients (unique User-Agents)

# Pids for cleanup
SERVER_PID=""
CONSUMER_PID=""

log() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

cleanup() {
    log "Cleaning up..."

    if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null || true
        log "Stopped server (PID: $SERVER_PID)"
    fi

    if [[ -n "$CONSUMER_PID" ]] && kill -0 "$CONSUMER_PID" 2>/dev/null; then
        kill "$CONSUMER_PID" 2>/dev/null || true
        log "Stopped consumer (PID: $CONSUMER_PID)"
    fi

    # Optionally stop docker-compose
    if [[ "${STOP_DOCKER:-false}" == "true" ]]; then
        docker-compose down 2>/dev/null || true
        log "Stopped docker-compose services"
    fi
}

trap cleanup EXIT

install_hey() {
    if ! command -v hey &> /dev/null; then
        log "Installing hey load testing tool..."
        go install github.com/rakyll/hey@latest
        success "hey installed"
    fi
}

wait_for_service() {
    local url=$1
    local name=$2
    local max_attempts=${3:-30}

    log "Waiting for $name to be ready..."
    for i in $(seq 1 $max_attempts); do
        if curl -s "$url" > /dev/null 2>&1; then
            success "$name is ready"
            return 0
        fi
        sleep 1
    done
    error "$name failed to start after $max_attempts seconds"
    return 1
}

start_infrastructure() {
    log "Starting infrastructure (Redis, TimescaleDB, migrations)..."
    docker-compose up -d

    # Wait for migrations to complete
    log "Waiting for migrations to complete..."
    for i in $(seq 1 60); do
        if docker-compose ps migrate 2>/dev/null | grep -q "exited"; then
            success "Migrations completed"
            break
        fi
        sleep 1
    done
}

build_and_start_services() {
    log "Building server..."
    go build -o bin/server ./cmd/server
    success "Server built"

    log "Building consumer..."
    go build -o bin/consumer ./cmd/consumer
    success "Consumer built"

    log "Starting server..."
    ./bin/server --database-url="$DATABASE_URL" > logs/server.log 2>&1 &
    SERVER_PID=$!
    log "Server started (PID: $SERVER_PID)"

    log "Starting consumer..."
    DATABASE_URL="$DATABASE_URL" REDIS_ADDR="${REDIS_ADDR:-localhost:6379}" \
        ./bin/consumer > logs/consumer.log 2>&1 &
    CONSUMER_PID=$!
    log "Consumer started (PID: $CONSUMER_PID)"

    wait_for_service "$BASE_URL/health" "Server"
}

run_health_check() {
    echo ""
    echo "=============================================="
    echo "  Health Check"
    echo "=============================================="
    curl -s "$BASE_URL/health" | jq . || curl -s "$BASE_URL/health"
    echo ""
}

run_write_test() {
    echo ""
    echo "=============================================="
    echo "  Write Test (POST /shorten)"
    echo "  Concurrency: $CONCURRENCY"
    echo "  Requests: $REQUESTS"
    echo "  Clients: $NUM_CLIENTS (unique User-Agents)"
    echo "=============================================="

    # Run parallel hey instances with different User-Agents to simulate multiple clients
    local pids=()
    local requests_per_client=$((REQUESTS / NUM_CLIENTS))

    for i in $(seq 1 "$NUM_CLIENTS"); do
        hey -n "$requests_per_client" -c "$((CONCURRENCY / NUM_CLIENTS + 1))" \
            -m POST \
            -H "Content-Type: application/json" \
            -H "User-Agent: StressTest-Client-$i/1.0" \
            -d '{"url":"https://example.com/test/path?param=value","strategy":"token"}' \
            "$BASE_URL/shorten" &
        pids+=($!)
    done

    # Wait for all to complete
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done
}

run_read_test() {
    # First create a URL to read
    log "Creating test URL for read test..."
    local response
    response=$(curl -s -X POST "$BASE_URL/shorten" \
        -H "Content-Type: application/json" \
        -H "User-Agent: StressTest-Setup/1.0" \
        -d '{"url":"https://example.com/read-test","strategy":"hash"}')

    local code
    code=$(echo "$response" | jq -r '.code' 2>/dev/null || echo "")

    if [[ -z "$code" || "$code" == "null" ]]; then
        error "Failed to create test URL"
        return 1
    fi

    success "Created test URL with code: $code"

    echo ""
    echo "=============================================="
    echo "  Read Test (GET /$code - redirect)"
    echo "  Concurrency: $CONCURRENCY"
    echo "  Requests: $REQUESTS"
    echo "  Clients: $NUM_CLIENTS (unique User-Agents)"
    echo "=============================================="

    # Run parallel hey instances with different User-Agents to simulate multiple clients
    local pids=()
    local requests_per_client=$((REQUESTS / NUM_CLIENTS))

    for i in $(seq 1 "$NUM_CLIENTS"); do
        hey -n "$requests_per_client" -c "$((CONCURRENCY / NUM_CLIENTS + 1))" \
            -H "User-Agent: StressTest-Client-$i/1.0" \
            "$BASE_URL/$code" &
        pids+=($!)
    done

    # Wait for all to complete
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done
}

run_mixed_test() {
    echo ""
    echo "=============================================="
    echo "  Mixed Workload Test"
    echo "  Duration: $DURATION"
    echo "  Concurrency: $CONCURRENCY"
    echo "  Clients: $NUM_CLIENTS (unique User-Agents)"
    echo "=============================================="

    # Create some URLs first using different clients
    log "Seeding test data..."
    local codes=()
    for i in $(seq 1 "$NUM_CLIENTS"); do
        local response
        response=$(curl -s -X POST "$BASE_URL/shorten" \
            -H "Content-Type: application/json" \
            -H "User-Agent: StressTest-Seed-$i/1.0" \
            -d "{\"url\":\"https://example.com/seed/$i\",\"strategy\":\"token\"}")
        local code
        code=$(echo "$response" | jq -r '.code' 2>/dev/null)
        if [[ -n "$code" && "$code" != "null" ]]; then
            codes+=("$code")
        fi
    done
    success "Created ${#codes[@]} seed URLs"

    # Run sustained load with multiple clients
    if [[ ${#codes[@]} -gt 0 ]]; then
        local test_code=${codes[0]}
        echo ""
        log "Running sustained read load for $DURATION with $NUM_CLIENTS clients..."

        local pids=()
        for i in $(seq 1 "$NUM_CLIENTS"); do
            hey -z "$DURATION" -c "$((CONCURRENCY / NUM_CLIENTS + 1))" \
                -H "User-Agent: StressTest-Client-$i/1.0" \
                "$BASE_URL/$test_code" &
            pids+=($!)
        done

        # Wait for all to complete
        for pid in "${pids[@]}"; do
            wait "$pid" 2>/dev/null || true
        done
    fi
}

run_rate_limit_test() {
    echo ""
    echo "=============================================="
    echo "  Rate Limit Test"
    echo "  Testing write limits (10/min default)"
    echo "=============================================="

    log "Sending 15 rapid write requests to trigger rate limiting..."
    local success_count=0
    local rate_limited_count=0

    for i in $(seq 1 15); do
        local http_code
        http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/shorten" \
            -H "Content-Type: application/json" \
            -d "{\"url\":\"https://example.com/rate-test/$i\",\"strategy\":\"token\"}")

        if [[ "$http_code" == "200" || "$http_code" == "201" ]]; then
            ((success_count++))
        elif [[ "$http_code" == "429" ]]; then
            ((rate_limited_count++))
        fi
    done

    echo "  Successful requests: $success_count"
    echo "  Rate limited (429): $rate_limited_count"

    if [[ $rate_limited_count -gt 0 ]]; then
        success "Rate limiting is working"
    else
        warn "No rate limiting triggered (may need to adjust limits or run faster)"
    fi
}

show_stats() {
    echo ""
    echo "=============================================="
    echo "  System Stats"
    echo "=============================================="

    if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "Server process:"
        ps -p "$SERVER_PID" -o pid,pcpu,pmem,rss,vsz,etime 2>/dev/null || true
    fi

    if [[ -n "$CONSUMER_PID" ]] && kill -0 "$CONSUMER_PID" 2>/dev/null; then
        echo "Consumer process:"
        ps -p "$CONSUMER_PID" -o pid,pcpu,pmem,rss,vsz,etime 2>/dev/null || true
    fi

    echo ""
    echo "Docker containers:"
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" 2>/dev/null || true
}

print_usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  all       Run all tests (default)"
    echo "  write     Run write (POST) load test only"
    echo "  read      Run read (GET) load test only"
    echo "  mixed     Run mixed workload test"
    echo "  ratelimit Test rate limiting"
    echo "  infra     Start infrastructure only"
    echo ""
    echo "Environment variables:"
    echo "  BASE_URL     Server URL (default: http://localhost:8888)"
    echo "  CONCURRENCY  Number of concurrent workers (default: 50)"
    echo "  REQUESTS     Number of requests for fixed tests (default: 1000)"
    echo "  DURATION     Duration for timed tests (default: 30s)"
    echo "  NUM_CLIENTS  Simulated clients with unique User-Agents (default: 10)"
    echo "  STOP_DOCKER  Stop docker-compose on exit (default: false)"
    echo ""
    echo "Examples:"
    echo "  $0                           # Run all tests with defaults"
    echo "  NUM_CLIENTS=50 $0 write      # 50 clients, write test only"
    echo "  REQUESTS=10000 $0 read       # 10k requests, read test"
}

main() {
    local command=${1:-all}

    echo ""
    echo "=============================================="
    echo "  URL Shortener Stress Test"
    echo "=============================================="
    echo ""

    # Create directories
    mkdir -p bin logs

    # Install dependencies
    install_hey

    case $command in
        infra)
            start_infrastructure
            ;;
        write|read|mixed|ratelimit|all)
            start_infrastructure
            build_and_start_services
            run_health_check

            case $command in
                write)
                    run_write_test
                    ;;
                read)
                    run_read_test
                    ;;
                mixed)
                    run_mixed_test
                    ;;
                ratelimit)
                    run_rate_limit_test
                    ;;
                all)
                    run_write_test
                    run_read_test
                    run_rate_limit_test
                    run_mixed_test
                    ;;
            esac

            show_stats
            ;;
        help|--help|-h)
            print_usage
            ;;
        *)
            error "Unknown command: $command"
            print_usage
            exit 1
            ;;
    esac

    echo ""
    success "Stress test completed!"
}

main "$@"
