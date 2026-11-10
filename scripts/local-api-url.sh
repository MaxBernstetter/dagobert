#!/usr/bin/env bash
# Print where the Vite dev server sends /tradingcopilot.v1.* on Floci: two lines, the origin and the path prefix.
#   line 1: http://127.0.0.1:<FLOCI_PORT>            (VITE_DEV_API_PROXY)
#   line 2: /_aws/execute-api/<api-id>/$default      (VITE_DEV_API_PREFIX)
# The api id is the first label of the host Terraform reports as api_gateway_url. Needs `task infra:local` first.
set -euo pipefail
cd "$(dirname "$0")/.."

URL="$(scripts/tf-output.sh api_gateway_url)"
if [ -z "$URL" ]; then
  echo "cannot read api_gateway_url from Terraform (stack not applied); run task infra:local first" >&2
  exit 1
fi

HOST="${URL#*://}"
HOST="${HOST%%[/:]*}"
API_ID="${HOST%%.*}"
if [ -z "$API_ID" ]; then
  echo "cannot read the API id from api_gateway_url: $URL" >&2
  exit 1
fi

echo "http://127.0.0.1:${FLOCI_PORT:-11566}"
echo "/_aws/execute-api/${API_ID}/\$default"
