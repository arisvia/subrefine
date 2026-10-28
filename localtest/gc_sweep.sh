#!/usr/bin/env bash
# Compare GC tunings under the same load; prints idle + plateau per config.
set -uo pipefail

kill_srv() {
  local p
  p=$(netstat -ano | grep "127.0.0.1:29999" | grep LISTENING | head -1 | awk '{print $NF}')
  [ -n "$p" ] && powershell -NoProfile -Command "Stop-Process -Id $p -Force" 2>/dev/null
  sleep 1
}

run_case() {
  local label="$1"; shift
  kill_srv
  env "$@" SUBREFINE_ADMIN_TOKEN=local-admin-secret-9x7 SUBREFINE_DATA_DIR=localtest/data \
    ./subrefine-test.exe -host 127.0.0.1 -port 29999 >localtest/srv.log 2>&1 &
  for _ in $(seq 1 40); do
    curl -fsS http://127.0.0.1:29999/healthz >/dev/null 2>&1 && break
    sleep 0.25
  done
  echo "---- $label"
  python localtest/mem_plateau.py | tail -4
}

run_case "default (GOGC=50, limit=12MiB from code)"
run_case "limit=8MiB GOGC=40" GOMEMLIMIT=8MiB GOGC=40
run_case "limit=6MiB GOGC=25" GOMEMLIMIT=6MiB GOGC=25
kill_srv
