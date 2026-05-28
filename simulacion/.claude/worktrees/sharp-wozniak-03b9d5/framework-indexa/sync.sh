#!/bin/bash
# sync.sh: sincroniza TimeBilling API -> dump.json -> SQLite -> store BM25
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

: "${PANALBIT_USER:?PANALBIT_USER requerido}"
: "${PANALBIT_PASSWORD:?PANALBIT_PASSWORD requerido}"
: "${PANALBIT_APP_KEY:=panalbit}"
: "${PANALBIT_BASE_URL:=https://panalbit.thetimebilling.com/time_tracking/api/v2}"

DUMP_PATH="${DUMP_PATH:-data/dump.json}"
SQLITE_PATH="${SQLITE_PATH:-data/panalbit.db}"
STORE_PATH="${STORE_PATH:-data/store.json}"
META_PATH="${META_PATH:-data/sync_meta.json}"
PAGE_SIZE="${PAGE_SIZE:-3000}"
CONCURRENCY="${CONCURRENCY:-4}"

echo "=== Indexa Sync $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
echo "API: $PANALBIT_BASE_URL"
echo "User: $PANALBIT_USER"

# Leer counts previos
PREV_COUNT=0
PREV_SYNC="never"
if [ -f "$META_PATH" ]; then
  PREV_COUNT=$(python3 -c "import json; d=json.load(open('$META_PATH')); print(d.get('total_records',0))" 2>/dev/null || echo 0)
  PREV_SYNC=$(python3 -c "import json; d=json.load(open('$META_PATH')); print(d.get('last_sync_at','never'))" 2>/dev/null || echo "never")
fi

echo "Previous: $PREV_COUNT records (synced: $PREV_SYNC)"

# 1. Descargar dump desde API
./panalbit-sync \
  --out "$DUMP_PATH" \
  --page-size "$PAGE_SIZE" \
  --concurrency "$CONCURRENCY"

# 2. Calcular counts actuales desde dump
CURRENT_COUNT=$(python3 -c "
import json
with open('$DUMP_PATH') as f:
    d = json.load(f)
endpoints = d.get('endpoints', {})
total = sum(len(v) for v in endpoints.values() if isinstance(v, list))
print(total)
")

echo "Current: $CURRENT_COUNT records"

# 3. Indexar a store BM25 + generar SQLite
./frameworkindexa index \
  --source "$DUMP_PATH" \
  --sqlite "$SQLITE_PATH"

# 4. Persistir metadata
python3 > "$META_PATH" <<PY
import json, sys, os
meta = {
    "last_sync_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "base_url": os.environ.get("PANALBIT_BASE_URL"),
    "user": os.environ.get("PANALBIT_USER"),
    "dump_path": "$DUMP_PATH",
    "sqlite_path": "$SQLITE_PATH",
    "store_path": "$STORE_PATH",
    "total_records": $CURRENT_COUNT,
    "previous_records": $PREV_COUNT,
    "changed": $CURRENT_COUNT != $PREV_COUNT,
    "page_size": $PAGE_SIZE,
    "concurrency": $CONCURRENCY,
}
json.dump(meta, sys.stdout, indent=2)
PY

echo ""
echo "=== Sync done ==="
if [ "$CURRENT_COUNT" != "$PREV_COUNT" ]; then
  echo "WARNING: record count changed ($PREV_COUNT -> $CURRENT_COUNT)"
  echo "Data was modified in the API since last sync."
else
  echo "Record count stable: $CURRENT_COUNT"
fi
echo "Meta: $META_PATH"
