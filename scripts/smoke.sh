#!/usr/bin/env bash
# Smoke test for ward-duty deployment.
#
# Usage:
#   bash scripts/smoke.sh https://ward-duty.dltmxm.link
#   bash scripts/smoke.sh http://localhost            # 홈서버 로컬 (Cloudflare 우회)

set -euo pipefail

BASE="${1:-https://ward-duty.dltmxm.link}"
echo "smoke target: $BASE"
echo "================"

pass() { printf "  \033[32m✓\033[0m %s\n" "$1"; }
fail() { printf "  \033[31m✗\033[0m %s\n" "$1"; FAILED=$((FAILED+1)); }
FAILED=0

# 1. Web 진입점 200
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/" || true)
if [[ "$status" == "200" || "$status" == "307" || "$status" == "302" ]]; then
  pass "GET /                          → $status (web 응답 또는 /login 리다이렉트)"
else
  fail "GET /                          → $status (200/307/302 기대)"
fi

# 2. /health (Go API)
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/health" || true)
if [[ "$status" == "200" ]]; then
  pass "GET /health                    → 200"
else
  fail "GET /health                    → $status (200 기대) — API 컨테이너 또는 Traefik 라우팅 확인"
fi

# 3. 보호 엔드포인트 401
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/auth/me" || true)
if [[ "$status" == "401" ]]; then
  pass "GET /api/auth/me (no cookie)   → 401 (auth middleware 동작)"
else
  fail "GET /api/auth/me (no cookie)   → $status (401 기대)"
fi

# 4. OAuth start가 Google로 302 리다이렉트
loc=$(curl -s -o /dev/null -w "%{http_code} %{redirect_url}" "$BASE/api/auth/oauth/google/start" || true)
if [[ "$loc" == 302* && "$loc" == *"accounts.google.com"* ]]; then
  pass "GET /api/auth/oauth/google/start → 302 → accounts.google.com (OAuth 시작 OK)"
else
  pass_or_fail="fail"
  if [[ "$loc" == 302* ]]; then pass_or_fail="warn"; fi
  fail "GET /api/auth/oauth/google/start → '$loc' (302 → accounts.google.com 기대)"
fi

# 5. Solver는 외부에서 보이면 안 됨
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/solver/health" || true)
if [[ "$status" == "404" ]]; then
  pass "GET /solver/health             → 404 (외부 노출 차단됨)"
else
  fail "GET /solver/health             → $status (404 기대 — 솔버가 외부에 노출됨!)"
fi

# 6. 잘못된 schedule create는 401 (head_nurse 인증 필요)
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/api/schedules" \
  -H "Content-Type: application/json" -d '{"year_month":"2099-12"}' || true)
if [[ "$status" == "401" ]]; then
  pass "POST /api/schedules (no auth)  → 401"
else
  fail "POST /api/schedules (no auth)  → $status (401 기대)"
fi

echo "================"
if [[ "$FAILED" -eq 0 ]]; then
  printf "\033[32mALL PASSED\033[0m\n"
  exit 0
else
  printf "\033[31m%d test(s) failed\033[0m\n" "$FAILED"
  exit 1
fi
