#!/usr/bin/env bash
# Print one output of the local stack's Terraform state (infra/terraform on Floci). Read-only: nothing is applied.
# Usage: scripts/tf-output.sh <output-name>
# Prints nothing and exits 0 when the stack has not been applied yet, so callers can test for an empty value.
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$PWD"

. scripts/lib/require-env.sh
. scripts/lib/terraform.sh

require_env AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION
FLOCI_ENDPOINT="http://127.0.0.1:${FLOCI_PORT:-11566}"
TF_NETWORK_HOST=1

if [ -z "$(tf_in "$ROOT/infra/bootstrap" output -raw state_bucket 2>/dev/null || true)" ]; then
  exit 0
fi
tf_backend_init local
tf_in "$ROOT/infra/terraform" output -raw "$1" 2>/dev/null || true
