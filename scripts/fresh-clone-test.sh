#!/usr/bin/env bash
# scripts/fresh-clone-test.sh
# Step 52: Run a fresh-clone quickstart verification.
#
# This script simulates what a new user does after cloning the repo.
# Run it from a clean checkout or CI environment.
#
# Prerequisites: Go 1.23+, Node.js 20+, Docker (with Compose V2)
#
# Usage:
#   git clone https://github.com/localcloud-dev/localcloud.git /tmp/lc-test
#   cd /tmp/lc-test
#   bash scripts/fresh-clone-test.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

passed=0
failed=0

ok()   { echo -e "  ${GREEN}✓${NC} $1"; ((passed++)); }
fail() { echo -e "  ${RED}✗${NC} $1: $2"; ((failed++)); }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; }

echo "LocalCloud Fresh-Clone Quickstart Test"
echo "======================================="
echo ""

# ─── 1. Check prerequisites ───
echo "1. Prerequisites"
command -v go   >/dev/null 2>&1 && ok "Go $(go version | awk '{print $3}')" || { fail "Go" "not installed"; exit 1; }
command -v node >/dev/null 2>&1 && ok "Node $(node --version)"             || { fail "Node" "not installed"; exit 1; }
command -v docker >/dev/null 2>&1 && ok "Docker available"                  || { fail "Docker" "not installed"; exit 1; }
docker compose version >/dev/null 2>&1 && ok "Compose V2"                   || { fail "Docker Compose" "not available"; exit 1; }

# ─── 2. Build CLI ───
echo ""
echo "2. Build CLI"
if make build 2>&1; then
    ok "make build succeeded"
    ./localcloud version && ok "Binary runs" || fail "Binary" "cannot execute"
else
    fail "make build" "build failed"
    exit 1
fi

# ─── 3. Install Studio deps and build ───
echo ""
echo "3. Studio build"
if make studio-install 2>&1 && make studio-build 2>&1; then
    ok "Studio build succeeded"
else
    fail "Studio" "build failed"
    exit 1
fi

# ─── 4. Run Go tests ───
echo ""
echo "4. Go tests"
if go test -race -count=1 ./... 2>&1; then
    ok "All Go tests pass"
else
    fail "Go tests" "some tests failed"
fi

# ─── 5. Studio type check ───
echo ""
echo "5. Studio type check"
if make studio-typecheck 2>&1; then
    ok "Studio type check passed"
else
    fail "Studio type check" "type errors found"
fi

# ─── 6. Init with demo ───
echo ""
echo "6. Init demo"
WORKDIR=$(mktemp -d)
pushd "$WORKDIR" > /dev/null
trap 'popd > /dev/null 2>&1; rm -rf "$WORKDIR"' EXIT

cp -r "$(dirs -l +1)/demo" ./demo
if "$(dirs -l +1)/localcloud" init --example demo-saas 2>&1; then
    ok "localcloud init --example demo-saas"
    [ -f localcloud.yml ] && ok "localcloud.yml created" || fail "Config" "localcloud.yml missing"
else
    fail "Init" "init failed"
fi

# ─── 7. Doctor ───
echo ""
echo "7. Doctor"
"$(dirs -l +1)/localcloud" doctor 2>&1
ok "Doctor ran (check output above)"

# ─── 8. Start stack ───
echo ""
echo "8. Start stack (docker compose up)"
docker compose -f demo/saas-app/docker-compose.yml up -d 2>&1
sleep 10  # Wait for services to be healthy

# Check services are healthy
for svc in postgres redis mailpit api worker; do
    if docker compose -f demo/saas-app/docker-compose.yml ps --format json | grep -q "\"$svc\""; then
        ok "$svc container running"
    else
        warn "$svc container status unknown"
    fi
done

# ─── 9. Start agent ───
echo ""
echo "9. Start agent"
"$(dirs -l +1)/localcloud" up --no-compose &
AGENT_PID=$!
sleep 3

if curl -sf http://127.0.0.1:41778/api/health > /dev/null 2>&1; then
    ok "Agent healthy"
else
    fail "Agent" "not responding on port 41778"
fi

# ─── 10. Run E2E test ───
echo ""
echo "10. E2E test"
if node "$(dirs -l +1)/scripts/e2e-test.js" 2>&1; then
    ok "E2E test passed"
else
    fail "E2E test" "some checks failed"
fi

# ─── 11. Cleanup ───
echo ""
echo "11. Cleanup"
kill $AGENT_PID 2>/dev/null || true
docker compose -f demo/saas-app/docker-compose.yml down -v 2>&1
ok "Stack stopped"

# ─── Summary ───
echo ""
echo "======================================="
echo -e "Results: ${GREEN}${passed} passed${NC}, ${RED}${failed} failed${NC}"
echo "======================================="
exit $((failed > 0 ? 1 : 0))
