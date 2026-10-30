#!/usr/bin/env bash
#
# Нагрузочный тест одного HTTP-эндпоинта (для сравнения производительности).
# Использует hey. Установка: go install github.com/rakyll/hey@latest
#
#   scripts/bench-http.sh <url> [concurrency] [duration]
#   scripts/bench-http.sh http://127.0.0.1:3000/api/prices/latest 50 10s
set -euo pipefail

URL="${1:?usage: $0 <url> [concurrency] [duration]}"
CONC="${2:-50}"
DUR="${3:-10s}"

if ! command -v hey >/dev/null 2>&1; then
  echo "hey не найден. Установи: go install github.com/rakyll/hey@latest" >&2
  exit 1
fi

echo "==> $URL (concurrency=$CONC, duration=$DUR)"
hey -c "$CONC" -z "$DUR" "$URL"
