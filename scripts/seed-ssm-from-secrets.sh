#!/usr/bin/env bash
# Push BYBIT_* from .secrets into Floci SSM (after task infra:local).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SECRETS_FILE="${HOST_SECRETS_FILE:-$ROOT/.secrets}"
FLOCI_PORT="${FLOCI_PORT:-4566}"
ENDPOINT="${AWS_ENDPOINT_URL:-http://127.0.0.1:${FLOCI_PORT}}"
PREFIX="${SSM_PREFIX:-/dagobert/local}"
REGION="${AWS_REGION:-us-east-1}"

export AWS_ENDPOINT_URL="$ENDPOINT"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="$REGION"

if [[ ! -f "$SECRETS_FILE" ]]; then
  echo "no secrets file at $SECRETS_FILE" >&2
  exit 1
fi

get_kv() {
  local key="$1"
  # Match KEY=value or export KEY=value; strip quotes.
  local line
  line="$(grep -E "^(export[[:space:]]+)?${key}=" "$SECRETS_FILE" | tail -1 || true)"
  [[ -z "$line" ]] && return 0
  local val="${line#*=}"
  val="${val%$'\r'}"
  if [[ "$val" == \"*\" ]]; then val="${val:1:-1}"; fi
  if [[ "$val" == \'*\' ]]; then val="${val:1:-1}"; fi
  printf '%s' "$val"
}

aws_cli() {
  if command -v aws >/dev/null 2>&1; then
    aws "$@"
    return
  fi
  # Same pattern as Taskfile infra:local — Docker AWS CLI when host has none.
  docker run --rm --network host \
    -e AWS_ENDPOINT_URL \
    -e AWS_ACCESS_KEY_ID \
    -e AWS_SECRET_ACCESS_KEY \
    -e AWS_DEFAULT_REGION \
    amazon/aws-cli:2.22.35 "$@"
}

put() {
  local name="$1" value="$2" type="${3:-SecureString}"
  [[ -z "$value" ]] && return 0
  aws_cli ssm put-parameter \
    --name "$name" \
    --type "$type" \
    --value "$value" \
    --overwrite \
    >/dev/null
  echo "put $name"
}

put "$PREFIX/BYBIT_API_KEY" "$(get_kv BYBIT_API_KEY)"
put "$PREFIX/BYBIT_API_SECRET" "$(get_kv BYBIT_API_SECRET)"
BASE="$(get_kv BYBIT_BASE_URL)"
put "$PREFIX/BYBIT_BASE_URL" "${BASE:-https://api-demo.bybit.eu}" String

echo "SSM seed complete → $PREFIX (endpoint $ENDPOINT)"
