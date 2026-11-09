#!/usr/bin/env bash
# Terraform apply for one environment. The Terraform code is identical; only the switch differs.
#   local (default): Floci. Fake credentials, every endpoint on Floci, cannot reach AWS.
#   prod:            real AWS. Refuses unless it runs in CI with credentials present.
set -euo pipefail
cd "$(dirname "$0")/.."

ENV_NAME="${1:-local}"
TF_IMG="hashicorp/terraform:1.9"
TF_ARGS=(-input=false -auto-approve "-var=environment=${ENV_NAME}")

case "$ENV_NAME" in
  local)
    FLOCI_ENDPOINT="http://127.0.0.1:${FLOCI_PORT:-11566}"
    docker compose up -d --wait floci 2>/dev/null || docker compose up -d floci
    for _ in $(seq 1 30); do curl -fsS "$FLOCI_ENDPOINT/_floci/health" >/dev/null 2>&1 && break; sleep 1; done
    TF_ARGS+=("-var=aws_region=us-east-1" "-var=floci_endpoint=${FLOCI_ENDPOINT}" "-var=database_port=7001"
              "-var=database_master_password=${POSTGRES_PASSWORD:-trading}")
    # A dev shell may hold real AWS credentials. Local must never see them.
    unset AWS_PROFILE AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_ENDPOINT_URL AWS_ROLE_ARN AWS_WEB_IDENTITY_TOKEN_FILE
    ;;
  prod)
    if [ "${CI:-}" != "true" ]; then
      echo "Refusing to deploy to real AWS outside CI (CI is not 'true'). Use 'task infra:local' for a dev setup." >&2
      exit 1
    fi
    if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -z "${AWS_ROLE_ARN:-}" ] && [ -z "${AWS_WEB_IDENTITY_TOKEN_FILE:-}" ]; then
      echo "Refusing to deploy: no AWS credentials in the CI environment." >&2
      exit 1
    fi
    export TF_VAR_allow_cloud_deploy=true
    TF_ARGS+=("-var=aws_region=${AWS_REGION:-eu-central-1}")
    ;;
  *)
    echo "Usage: $0 local|prod" >&2
    exit 2
    ;;
esac

if command -v terraform >/dev/null 2>&1; then
  (cd infra/terraform && terraform init -input=false && terraform apply "${TF_ARGS[@]}")
else
  NET=(); [ "$ENV_NAME" = "local" ] && NET=(--network host)
  RUN=(docker run --rm "${NET[@]}" --user "$(id -u):$(id -g)" -e HOME=/tmp -e TF_VAR_allow_cloud_deploy
       -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN -e AWS_REGION -e AWS_ROLE_ARN -e AWS_WEB_IDENTITY_TOKEN_FILE
       -v "$PWD/infra/terraform:/tf" -w /tf "$TF_IMG")
  "${RUN[@]}" init -input=false
  "${RUN[@]}" apply "${TF_ARGS[@]}"
fi
