#!/usr/bin/env bash
# Build the SPA for the same origin (VITE_EXPERIMENT_API_URL empty: the browser calls /tradingcopilot.v1.*
# on the CloudFront domain) and upload dist/ to the frontend bucket that Terraform created.
#   local (default in infra-apply): Floci on FLOCI_PORT (default 11566), credentials from .env.
#   prod:                           real AWS, CI only (same guard as infra-apply.sh).
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/lib/require-env.sh

ENV_NAME="${1:-}"
AWS_IMG="amazon/aws-cli:2.39.5"
NODE_IMG="node:22-alpine"
ROOT="$PWD"
DIST="$ROOT/src/frontend/dist"
HOST_IDS="$(id -u):$(id -g)"

case "$ENV_NAME" in
  local)
    FLOCI_ENDPOINT="http://127.0.0.1:${FLOCI_PORT:-11566}"
    TF_NETWORK_HOST=1
    require_env AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION
    # A dev shell may hold a profile or role. Local uses the keys from .env and never a real endpoint.
    unset AWS_PROFILE AWS_SESSION_TOKEN AWS_ENDPOINT_URL AWS_ROLE_ARN AWS_WEB_IDENTITY_TOKEN_FILE
    export AWS_DEFAULT_REGION="$AWS_REGION"
    ENDPOINT_ARGS=(--endpoint-url "$FLOCI_ENDPOINT")
    NET=(--network host)
    ;;
  prod)
    if [ "${CI:-}" != "true" ]; then
      echo "Refusing to deploy the web app to real AWS outside CI (CI is not 'true')." >&2
      exit 1
    fi
    require_env AWS_REGION
    ENDPOINT_ARGS=()
    NET=()
    ;;
  *)
    echo "Usage: $0 local|prod" >&2
    exit 2
    ;;
esac

# tf_output NAME: a Terraform output of the state in the S3 backend (infra/terraform, initialised here).
. scripts/lib/terraform.sh
tf_init_root "$ENV_NAME"
tf_output() { tf_in "$ROOT/infra/terraform" output -raw "$1"; }

# Build. npm ci keeps the lockfile exact; the build reads the wiki markdown under src/calculation-*,
# so the whole repository is in scope, as in src/frontend/Dockerfile.
rm -rf "$DIST"
if command -v npm >/dev/null 2>&1; then
  (cd "$ROOT/src/frontend" && npm ci && VITE_EXPERIMENT_API_URL= npm run build)
else
  docker run --rm --user "$HOST_IDS" -e HOME=/tmp -e VITE_EXPERIMENT_API_URL= \
    -v "$ROOT:/repo" -w /repo/src/frontend "$NODE_IMG" sh -c "npm ci && npm run build"
fi
[ -f "$DIST/index.html" ] || { echo "Build produced no dist/index.html" >&2; exit 1; }

# AWS CLI: the installed binary, else the aws-cli image with dist/ mounted read-only.
if command -v aws >/dev/null 2>&1; then
  AWS=(aws)
  SRC="$DIST"
else
  AWS=(docker run --rm "${NET[@]}" -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN
       -e AWS_DEFAULT_REGION -e AWS_REGION -e AWS_ROLE_ARN -e AWS_WEB_IDENTITY_TOKEN_FILE
       -v "$DIST:/dist:ro" "$AWS_IMG")
  SRC=/dist
fi
run_aws() { "${AWS[@]}" "${ENDPOINT_ARGS[@]}" "$@"; }

BUCKET="$(tf_output frontend_bucket)"

# Order matters: hashed assets first, then the other root files, then index.html, so the shell never
# points at assets that are not uploaded yet. Stale hashed files are left in place (no --delete).
run_aws s3 sync "$SRC/assets" "s3://$BUCKET/assets" \
  --cache-control "public,max-age=31536000,immutable" --only-show-errors
run_aws s3 sync "$SRC" "s3://$BUCKET" --exclude "index.html" --exclude "assets/*" \
  --cache-control "no-cache" --only-show-errors
run_aws s3 cp "$SRC/index.html" "s3://$BUCKET/index.html" \
  --cache-control "no-cache" --content-type "text/html; charset=utf-8" --only-show-errors

if [ "$ENV_NAME" = prod ]; then
  DIST_ID="$(tf_output frontend_distribution_id)"
  run_aws cloudfront create-invalidation --distribution-id "$DIST_ID" --paths '/index.html' >/dev/null
fi

echo "Uploaded the web app to s3://$BUCKET ($ENV_NAME)."
