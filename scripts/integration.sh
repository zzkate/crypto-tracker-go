#!/usr/bin/env bash
#
# Ручной интеграционный прогон с реальным CoinMarketCap.
# Требует CMC_API_KEY, APP_API_KEY и DATABASE_URL в окружении. Ключи не печатаются.
set -euo pipefail

: "${CMC_API_KEY:?CMC_API_KEY is required}"
: "${APP_API_KEY:?APP_API_KEY is required}"
: "${DATABASE_URL:?DATABASE_URL is required}"

BIN="${BIN:-/tmp/crypto-tracker-go}"
PORT="${PORT:-3000}"
SYMBOLS="${SYMBOLS:-BTC,ETH}"
CURRENCY="${PRICE_CURRENCY:-USD}"
BASE="http://127.0.0.1:${PORT}"

PORT="$PORT" APP_API_KEY="$APP_API_KEY" CMC_API_KEY="$CMC_API_KEY" \
  PRICE_CURRENCY="$CURRENCY" DATABASE_URL="$DATABASE_URL" \
  "$BIN" >/tmp/crypto-integration.log 2>&1 &
PID=$!
cleanup() {
  kill "$PID" 2>/dev/null || true
  wait "$PID" 2>/dev/null || true
}
trap cleanup EXIT

for _ in $(seq 1 60); do
  curl -s -o /dev/null "$BASE/health" && break || sleep 0.2
done
if ! curl -s -o /dev/null "$BASE/health"; then
  echo "сервис не поднялся"
  cat /tmp/crypto-integration.log
  exit 1
fi

AUTH="Authorization: Bearer $APP_API_KEY"
IFS=',' read -ra COINS <<< "$SYMBOLS"
for raw in "${COINS[@]}"; do
  sym="$(printf '%s' "$raw" | tr '[:lower:]' '[:upper:]' | xargs)"
  [ -z "$sym" ] && continue
  code=$(curl -s -o /tmp/body -w '%{http_code}' -X POST "$BASE/api/coins" \
    -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"symbol\":\"$sym\",\"name\":\"$sym\"}")
  [ "$code" = "201" ] || { echo "POST coin $sym -> $code $(cat /tmp/body)"; exit 1; }
  echo "монета добавлена: $sym"
done

sync_code=$(curl -s -o /tmp/body -w '%{http_code}' -X POST "$BASE/api/sync" -H "$AUTH")
echo "POST /api/sync -> HTTP $sync_code: $(cat /tmp/body)"
[ "$sync_code" = "200" ] || exit 1

curl -s "$BASE/api/prices/latest" -H "$AUTH" > /tmp/prices.json
python3 - <<'PY'
import json
prices = json.load(open('/tmp/prices.json'))
assert prices, 'empty prices'
for p in prices:
    assert isinstance(p.get('price'), (int, float)) and p['price'] > 0, p
    print(f"{p['symbol']}: {p['price']} {p['currency']} at {p['fetchedAt']}")
print('интеграционный прогон успешен')
PY
