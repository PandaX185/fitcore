#!/usr/bin/env bash
# report-tests.sh — full test report: coverage, race detection, pass/fail
# counts + reasons, and smoke flows against the live stack.
#
# Usage: scripts/report-tests.sh [--export [DIR]] [--help]
#
#   --export [DIR]   additionally write the report to DIR/test-report-<ts>.txt
#                    (DIR defaults to "artifacts")
#   --help           print this help
#
# What it runs, in order:
#   1. go test -tags integration -race -count=1 -coverprofile -json ./...
#      (integration tags include the plain unit suite; requires
#      TEST_DATABASE_URL / TEST_REDIS_URL to point at live services)
#   2. seeds the smoke staff accounts (just seed-smoke)
#   3. the health/auth/branches/members smoke flows against $SMOKE_BASE
#
# Environment:
#   SMOKE_BASE (default http://localhost:8080) — base URL of the running API
#
# Prerequisites: live dev stack (just compose-up), migrations applied
# (just migrate-up), API reachable at $SMOKE_BASE.
#
# Exit status: 0 only when every test passed, no races were detected, and
# every smoke flow passed. The full report is still printed before exiting.
set -uo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SELF_DIR/.." && pwd)"
SMOKE_BASE="${SMOKE_BASE:-http://localhost:8080}"

export_mode=0
export_dir="artifacts"

usage() {
    sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --export)
            export_mode=1
            if [[ $# -gt 1 && "$2" != -* ]]; then export_dir="$2"; shift; fi
            shift
            ;;
        --export=*)
            export_mode=1
            export_dir="${1#*=}"
            shift
            ;;
        -h | --help)
            usage
            exit 0
            ;;
        *)
            echo "report-tests: unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if ! command -v jq >/dev/null 2>&1; then
    echo "report-tests: jq is required (install jq)" >&2
    exit 2
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

REPORT="$TMP/report.txt"
RESULTS="$TMP/results.json"
COVER="$TMP/cover.out"
RC=0
SECONDS=0

# --- 1. Go tests (unit + integration, race, coverage) ------------------------
echo "== go test -count=1 -race -coverprofile cover.out -json ./... -tags integration =="
(
    cd "$ROOT_DIR" || exit 1
    go test -count=1 -race -coverprofile="$COVER" -json -tags integration ./...
) >"$RESULTS" 2>&1
GO_STATUS=$?
GO_SECS=$SECONDS

passed=$(jq -r 'select(.Action == "pass"  and .Test != null) | .Test' "$RESULTS" | wc -l | tr -d ' ')
skipped=$(jq -r 'select(.Action == "skip"  and .Test != null) | .Test' "$RESULTS" | wc -l | tr -d ' ')

# Count only leaf failures: a failing parent of failing subtests is implied by
# its children and would double-count the same defect.
leaf_fails=$(
    jq -sr '
        def leaf($t; $fs): ([$fs[] | select(. != $t and (. | startswith($t + "/")))] | length) == 0;
        ([.[] | select(.Action == "fail" and .Test != null) | .Test] | unique) as $fs
        | [$fs[] | select(leaf(.; $fs))]
    ' "$RESULTS"
)
fail_count=$(printf '%s' "$leaf_fails" | jq 'length' 2>/dev/null || echo 0)

pkg_ok=$(jq -r 'select(.Action == "pass" and .Test == null)              | .Package' "$RESULTS" | sort -u | wc -l | tr -d ' ')
pkg_fail=$(jq -r 'select(.Action == "fail"  and .Test == null and .FailedBuild == null) | .Package' "$RESULTS" | sort -u)
build_fail=$(jq -r 'select(.Action == "build-fail")                       | .Package' "$RESULTS" | sort -u)
pkg_fail_count=$(printf '%s\n' "$pkg_fail" "$build_fail" | sed '/^$/d' | sort -u | wc -l | tr -d ' ')

races=$(grep -c 'WARNING: DATA RACE' "$RESULTS" || true)

total="n/a"
if [[ -s "$COVER" ]]; then
    total=$(go tool cover -func="$COVER" | awk '/^total:/ { sub("%", "", $NF); print $NF }')
fi

{
    echo "FitCore test report"
    echo "==================="
    echo "generated: $(date '+%Y-%m-%d %H:%M:%S %z')"
    echo "go tests : $GO_SECS s (count=1, race on, tags: integration)"
    echo
    echo "go test results: passed=$passed failed=$fail_count skipped=$skipped"
    echo "packages        : $pkg_ok passed, $pkg_fail_count failed"
    echo "data races      : $races"
    echo "total coverage  : $total% (statements, tested packages only)"
    echo

    cov_lines=$(jq -r 'select(.Action == "output" and (.Output | startswith("coverage: "))) | [.Package, ((.Output | tostring) | capture("coverage: (?<p>[0-9.]+)%").p)] | @tsv' "$RESULTS" | sed 's#github.com/PandaX185/fitcore/##' | sort -k2,2rn)
    if [[ -n "$cov_lines" ]]; then
        echo "coverage by package:"
        while IFS=$'\t' read -r pkg pct; do
            printf '  %-38s %6s%%\n' "$pkg" "$pct"
        done <<<"$cov_lines"
        echo
    fi

    if [[ -n "$build_fail" ]]; then
        echo "packages that failed to build:"
        printf '  %s\n' $build_fail
        echo
    fi

    if [[ "$fail_count" -gt 0 ]]; then
        echo "---- FAILED TESTS ----"
        while IFS= read -r t; do
            echo "--- $t"
            jq -r --arg T "$t" 'select(.Test == $T and .Action == "output") | .Output' "$RESULTS"
        done < <(printf '%s' "$leaf_fails" | jq -r '.[]')
        echo
    fi

    if [[ "$races" -gt 0 ]]; then
        echo "---- DATA RACES ----"
        jq -r 'select(.Action == "output" and (.Output | contains("DATA RACE"))) | .Output' "$RESULTS"
        echo
    fi
} >>"$REPORT"

# --- 2. Seed smoke accounts --------------------------------------------------
echo "== seed-smoke accounts =="
if ! (cd "$ROOT_DIR" && just seed-smoke >/dev/null 2>&1); then
    echo "report-tests: \`just seed-smoke\` failed — is the DB up and migrated?" >&2
    RC=1
fi

# --- 3. Smoke flows ----------------------------------------------------------
run_flow() {
    local name="$1"
    shift
    { (cd "$ROOT_DIR" && "$@"); } 2>&1 | tee -a "$REPORT"
    local st=${PIPESTATUS[0]}
    [[ $st -ne 0 ]] && RC=1
    return "$st"
}

SECONDS=0
{
    echo
    echo "---- SMOKE FLOWS ----"
} >>"$REPORT"
run_flow health_flow "$ROOT_DIR/scripts/smoke/health_flow.sh"
run_flow auth_flow "$ROOT_DIR/scripts/smoke/auth_flow.sh"
run_flow branches_flow "$ROOT_DIR/scripts/smoke/branches_flow.sh"
run_flow members_flow "$ROOT_DIR/scripts/smoke/members_flow.sh"
SMOKE_SECS=$SECONDS

# --- footer ------------------------------------------------------------------
[[ $races -gt 0 ]] && RC=1
[[ $GO_STATUS -ne 0 ]] && RC=1

{
    echo
    echo "==================="
    echo "summary: tests passed=$passed failed=$fail_count skipped=$skipped | packages failed=$pkg_fail_count | races=$races | total coverage=${total}%"
    echo "go tests: ${GO_SECS}s   smoke: ${SMOKE_SECS}s   total: $((GO_SECS + SMOKE_SECS))s"
    if [[ $RC -eq 0 ]]; then
        echo "RESULT: PASS"
    else
        echo "RESULT: FAIL"
    fi
} >>"$REPORT"

cat "$REPORT"
echo

if [[ $export_mode -eq 1 ]]; then
    ts=$(date +%Y%m%d-%H%M%S)
    mkdir -p "$export_dir"
    dest="$export_dir/test-report-$ts.txt"
    cp "$REPORT" "$dest"
    echo "report exported to $dest"
fi

exit "$RC"