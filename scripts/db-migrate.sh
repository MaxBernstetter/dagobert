#!/usr/bin/env bash
# Run the Atlas migrations as a one-off ECS Fargate task inside the VPC (Aurora is private).
# Terraform (infra/terraform/db_migrate.tf) must already exist: `task infra:local` or scripts/infra-apply.sh prod.
#   local (default): Floci, credentials from .env, cannot reach AWS. The image is built under its ECR URI, not pushed.
#   prod:            real AWS, CI only. Refuses unless CI=true and AWS credentials are present; pushes the image.
# Exit 0 only if the container exited with code 0. Anything else (task did not stop, no exit code, exit != 0) fails.
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/lib/require-env.sh

ENV_NAME="${1:-local}"
AWS_IMG="amazon/aws-cli:2.39.5"
ROOT="$PWD"
HOST_IDS="$(id -u):$(id -g)"

case "$ENV_NAME" in
  local)
    FLOCI_ENDPOINT="http://127.0.0.1:${FLOCI_PORT:-11566}"
    TF_NETWORK_HOST=1
    require_env AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION
    # A dev shell may hold a profile or role. Local uses the keys from .env and never a real endpoint.
    unset AWS_PROFILE AWS_SESSION_TOKEN AWS_ENDPOINT_URL AWS_ROLE_ARN AWS_WEB_IDENTITY_TOKEN_FILE
    REGION="$AWS_REGION"
    ENDPOINT_ARGS=(--endpoint-url "$FLOCI_ENDPOINT")
    NET=(--network host)
    ;;
  prod)
    if [ "${CI:-}" != "true" ]; then
      echo "Refusing to run migrations against real AWS outside CI (CI is not 'true')." >&2
      exit 1
    fi
    if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -z "${AWS_ROLE_ARN:-}" ] && [ -z "${AWS_WEB_IDENTITY_TOKEN_FILE:-}" ]; then
      echo "Refusing to run migrations: no AWS credentials in the CI environment." >&2
      exit 1
    fi
    require_env AWS_REGION
    REGION="$AWS_REGION"
    ENDPOINT_ARGS=()
    NET=()
    ;;
  *)
    echo "Usage: $0 local|prod" >&2
    exit 2
    ;;
esac
export AWS_DEFAULT_REGION="$REGION"

# tf_output NAME: a Terraform output of the state in the S3 backend (infra/terraform, initialised here).
. scripts/lib/terraform.sh
tf_init_root "$ENV_NAME"
tf_output() { tf_in "$ROOT/infra/terraform" output -raw "$1"; }

# AWS CLI: the installed binary, else the aws-cli image (stdin kept open for docker login).
if command -v aws >/dev/null 2>&1; then
  AWS=(aws)
else
  AWS=(docker run --rm -i "${NET[@]}" -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN
       -e AWS_DEFAULT_REGION -e AWS_ROLE_ARN -e AWS_WEB_IDENTITY_TOKEN_FILE "$AWS_IMG")
fi
run_aws() { "${AWS[@]}" "${ENDPOINT_ARGS[@]}" "$@"; }

REPO_URL="$(tf_output db_migrate_repository_url)"
CLUSTER="$(tf_output db_migrate_cluster)"
FAMILY="$(tf_output db_migrate_task_family)"
SUBNETS="$(tf_output db_migrate_subnet_ids)"
SECURITY_GROUP="$(tf_output db_migrate_security_group_id)"
LOG_GROUP="$(tf_output db_migrate_log_group)"

# Build the image under the repository URI. Prod pushes it; Floci runs the local image by URI.
docker build -f docker/db-migrate/Dockerfile -t "${REPO_URL}:latest" .
if [ "$ENV_NAME" = prod ]; then
  run_aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "${REPO_URL%%/*}"
  docker push "${REPO_URL}:latest"
fi

# Start the task in the private subnets with the app security group. assignPublicIp is DISABLED.
# One run-task call only: a second call would start a second migration.
RUN_JSON="$(run_aws ecs run-task --cluster "$CLUSTER" --task-definition "$FAMILY" --launch-type FARGATE --count 1 \
  --network-configuration "awsvpcConfiguration={subnets=[${SUBNETS}],securityGroups=[${SECURITY_GROUP}],assignPublicIp=DISABLED}" \
  --output json)"
TASK_ARN="$(printf '%s\n' "$RUN_JSON" | sed -n 's/.*"taskArn": "\([^"]*\)".*/\1/p' | head -n 1)"
if [ -z "$TASK_ARN" ]; then
  echo "db-migrate: run-task started no task:" >&2
  printf '%s\n' "$RUN_JSON" >&2
  exit 1
fi
TASK_ID="${TASK_ARN##*/}"
echo "db-migrate: started task ${TASK_ID} on ${CLUSTER} (${ENV_NAME})"

# Wait for STOPPED (the waiter gives up after about 10 minutes). Read the logs and the exit code either way.
WAIT_RC=0
run_aws ecs wait tasks-stopped --cluster "$CLUSTER" --tasks "$TASK_ARN" || WAIT_RC=$?

describe() {
  run_aws ecs describe-tasks --cluster "$CLUSTER" --tasks "$TASK_ARN" --query "tasks[0].$1" --output text
}
LAST_STATUS="$(describe lastStatus)"
EXIT_CODE="$(describe 'containers[0].exitCode')"
STOP_CODE="$(describe stopCode)"
STOP_REASON="$(describe stoppedReason)"

echo "db-migrate: container log (${LOG_GROUP}, stream prefix migrate/db-migrate/${TASK_ID})"
run_aws logs tail "$LOG_GROUP" --log-stream-name-prefix "migrate/db-migrate/${TASK_ID}" --since 2h || \
  echo "db-migrate: could not read the log group ${LOG_GROUP}" >&2

if [ "$WAIT_RC" -ne 0 ] || [ "$LAST_STATUS" != STOPPED ]; then
  echo "db-migrate: task did not stop (lastStatus=${LAST_STATUS}, waiter exit ${WAIT_RC})" >&2
  exit 1
fi
# Floci may not report a container exit code. An empty or None value counts as failure, never as success.
if [ -z "$EXIT_CODE" ] || [ "$EXIT_CODE" = None ]; then
  echo "db-migrate: no container exit code (stopCode=${STOP_CODE}, stoppedReason=${STOP_REASON})" >&2
  exit 1
fi
if [ "$EXIT_CODE" != 0 ]; then
  echo "db-migrate: migration container exited ${EXIT_CODE} (stopCode=${STOP_CODE}, stoppedReason=${STOP_REASON})" >&2
  exit 1
fi
echo "db-migrate: migrations applied (${ENV_NAME})"
