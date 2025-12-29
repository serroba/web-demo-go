#!/usr/bin/env bash
set -euo pipefail

# Configuration
CURRENT_FILE="${1:-perf-results.json}"
BASELINE_FILE="${2:-baseline-perf.json}"
REGRESSION_THRESHOLD="${REGRESSION_THRESHOLD:-50}"  # 50% regression threshold (accounts for CI variability)
OUTPUT_FILE="${OUTPUT_FILE:-perf-report.md}"

# Check if current results exist
if [[ ! -f "$CURRENT_FILE" ]]; then
    echo "Error: Current results file not found: $CURRENT_FILE" >&2
    exit 1
fi

# Check if baseline exists
if [[ ! -f "$BASELINE_FILE" ]]; then
    echo "::notice::No baseline found. Current results will become baseline."
    cp "$CURRENT_FILE" "$BASELINE_FILE"

    # Generate report without comparison
    {
        echo "# Performance Results"
        echo ""
        echo "> No previous baseline to compare against. This run establishes the baseline."
        echo ""
        echo "## Current Results"
        echo ""
        echo "| Test | p95 (ms) | p99 (ms) | Avg (ms) | RPS |"
        echo "|------|----------|----------|----------|-----|"
        jq -r '.[] | "| \(.test) | \(.p95_ms) | \(.p99_ms) | \(.avg_ms) | \(.rps) |"' "$CURRENT_FILE"
        echo ""
        echo "---"
        echo ""
        echo "*Threshold: p95 < 50ms*"
    } > "$OUTPUT_FILE"

    echo "Report written to $OUTPUT_FILE"
    exit 0
fi

# Generate comparison report
has_regression=false

{
    echo "# Performance Comparison Report"
    echo ""
    echo "| Test | Baseline p95 | Current p95 | Change | Status |"
    echo "|------|--------------|-------------|--------|--------|"

    # Get all test names from current results
    while IFS= read -r test; do
        baseline_p95=$(jq -r ".[] | select(.test == \"$test\") | .p95_ms" "$BASELINE_FILE" 2>/dev/null || echo "")
        current_p95=$(jq -r ".[] | select(.test == \"$test\") | .p95_ms" "$CURRENT_FILE")

        if [[ -n "$baseline_p95" && "$baseline_p95" != "null" && "$baseline_p95" != "" ]]; then
            # Calculate percentage change
            change=$(echo "scale=2; (($current_p95 - $baseline_p95) / $baseline_p95) * 100" | bc -l 2>/dev/null || echo "0")

            # Format change with sign
            if (( $(echo "$change >= 0" | bc -l) )); then
                change_fmt="+${change}%"
            else
                change_fmt="${change}%"
            fi

            # Determine status
            if (( $(echo "$change > $REGRESSION_THRESHOLD" | bc -l) )); then
                status=":x: Regression"
                has_regression=true
            elif (( $(echo "$change < -5" | bc -l) )); then
                status=":rocket: Improved"
            else
                status=":white_check_mark: OK"
            fi
        else
            change_fmt="N/A"
            status=":new: New test"
            baseline_p95="N/A"
        fi

        echo "| $test | ${baseline_p95}ms | ${current_p95}ms | $change_fmt | $status |"
    done < <(jq -r '.[].test' "$CURRENT_FILE")

    echo ""
    echo "---"
    echo ""
    echo "**Configuration:**"
    echo "- p95 threshold: 50ms"
    echo "- Regression threshold: ${REGRESSION_THRESHOLD}%"
    echo ""

    # Add detailed results
    echo "<details>"
    echo "<summary>Detailed Results</summary>"
    echo ""
    echo "### Current Run"
    echo ""
    echo "| Test | p95 | p99 | Avg | RPS | Requests | Concurrency |"
    echo "|------|-----|-----|-----|-----|----------|-------------|"
    jq -r '.[] | "| \(.test) | \(.p95_ms)ms | \(.p99_ms)ms | \(.avg_ms)ms | \(.rps) | \(.requests) | \(.concurrency) |"' "$CURRENT_FILE"
    echo ""
    echo "</details>"
    echo ""

    if [[ "$has_regression" == "true" ]]; then
        echo ":warning: **Performance regression detected!** Please investigate before merging."
    else
        echo ":white_check_mark: **All performance checks passed.**"
    fi

} > "$OUTPUT_FILE"

echo "Report written to $OUTPUT_FILE"

# Exit with error if regression detected
if [[ "$has_regression" == "true" ]]; then
    echo "::error::Performance regression detected! See report for details."
    exit 1
fi

echo "No regressions detected."
