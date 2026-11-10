#!/usr/bin/env bash
# Start one backfill execution (ingest, then indicators) and wait for it to finish.
# Usage: scripts/backfill-start.sh <local|prod> '<json input>'
#   input: {"provider":"binance","base":"BTC","quote":"USDT","from":<ms>,"to":<ms>}
#   local: Floci (FLOCI_PORT, default 11566), credentials from .env, never real AWS.
#   prod:  real AWS, credentials and AWS_REGION from the CI environment.
# Prints the execution output as JSON. Exit 0 when the execution SUCCEEDED, 1 otherwise.
set -euo pipefail
. "$(dirname "$0")/lib/require-env.sh"

ENV_NAME="${1:-}"
INPUT="${2:-}"
if [ -z "$ENV_NAME" ] || [ -z "$INPUT" ]; then
  echo "Usage: $0 local|prod '<json input>'" >&2
  exit 2
fi

case "$ENV_NAME" in
  local)
    export AWS_ENDPOINT_URL="http://127.0.0.1:${FLOCI_PORT:-11566}"
    require_env AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION
    export AWS_DEFAULT_REGION="$AWS_REGION"
    # A dev shell may hold a profile or role. Local uses the keys from .env and never a real endpoint.
    unset AWS_PROFILE AWS_SESSION_TOKEN AWS_ROLE_ARN AWS_WEB_IDENTITY_TOKEN_FILE
    ;;
  prod)
    unset AWS_ENDPOINT_URL
    require_env AWS_REGION
    export AWS_DEFAULT_REGION="$AWS_REGION"
    ;;
  *)
    echo "Usage: $0 local|prod '<json input>'" >&2
    exit 2
    ;;
esac

if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI is required" >&2
  exit 1
fi

# Validate the input before it reaches the state machine.
if ! printf '%s' "$INPUT" | python3 -I -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d,dict)' 2>/dev/null; then
  echo "input is not a JSON object" >&2
  exit 2
fi

NAME="dagobert-${ENV_NAME}-backfill"
ARN="$(aws stepfunctions list-state-machines --query "stateMachines[?name=='${NAME}'].stateMachineArn | [0]" --output text)"
if [ -z "$ARN" ] || [ "$ARN" = "None" ]; then
  echo "state machine ${NAME} not found; run task infra:${ENV_NAME} first" >&2
  exit 1
fi

EXEC_ARN="$(aws stepfunctions start-execution --state-machine-arn "$ARN" --input "$INPUT" --query executionArn --output text)"
echo "started ${EXEC_ARN}" >&2

STATUS="RUNNING"
while [ "$STATUS" = "RUNNING" ]; do
  sleep "${BACKFILL_POLL_SECONDS:-15}"
  STATUS="$(aws stepfunctions describe-execution --execution-arn "$EXEC_ARN" --query status --output text)"
done

aws stepfunctions describe-execution --execution-arn "$EXEC_ARN" --query '{status: status, output: output, error: error, cause: cause}' --output json
if [ "$STATUS" = "SUCCEEDED" ]; then
  exit 0
fi
echo "backfill ${STATUS}" >&2
exit 1
