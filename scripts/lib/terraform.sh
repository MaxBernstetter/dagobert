# Terraform for the scripts: the installed binary if present, else the pinned image. Sourced from the repo root;
# expects ROOT (repo root), AWS_REGION and, for local, FLOCI_ENDPOINT.

TF_IMAGE="${TF_IMAGE:-hashicorp/terraform:1.10.5}"

# tf_in DIR ARGS...: run terraform in DIR. Credentials and region pass through from the environment.
tf_in() {
  local dir="$1"
  shift
  if command -v terraform >/dev/null 2>&1; then
    (cd "$dir" && terraform "$@")
    return
  fi
  local net=()
  [ "${TF_NETWORK_HOST:-0}" = 1 ] && net=(--network host)
  docker run --rm "${net[@]}" --user "$(id -u):$(id -g)" -e HOME=/tmp \
    -e TF_VAR_allow_cloud_deploy \
    -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN -e AWS_REGION -e AWS_DEFAULT_REGION \
    -e AWS_ROLE_ARN -e AWS_WEB_IDENTITY_TOKEN_FILE -e AWS_ENDPOINT_URL_S3 \
    -v "$dir:/tf" -w /tf "$TF_IMAGE" "$@"
}

# tf_backend_init ENV: init infra/terraform against the state bucket that infra/bootstrap created (no apply).
# Local points the S3 backend at Floci; prod uses the default chain.
tf_backend_init() {
  local env="$1"
  local backend=(
    "-backend-config=region=${AWS_REGION}"
    "-backend-config=key=dagobert/${env}/terraform.tfstate"
    "-backend-config=use_lockfile=true"
  )
  if [ "$env" = local ]; then
    backend+=(
      -backend-config=skip_credentials_validation=true
      -backend-config=skip_metadata_api_check=true
      -backend-config=skip_requesting_account_id=true
      -backend-config=force_path_style=true
    )
    export AWS_ENDPOINT_URL_S3="${FLOCI_ENDPOINT}"
  fi
  local bucket
  bucket="$(tf_in "$ROOT/infra/bootstrap" output -raw state_bucket)"
  tf_in "$ROOT/infra/terraform" init -input=false -reconfigure -no-color \
    "-backend-config=bucket=${bucket}" "${backend[@]}" >/dev/null
}

# tf_init_root ENV: apply the state bucket (infra/bootstrap, local state), then init infra/terraform on it.
# Re-running is a no-op when the bucket is unchanged.
tf_init_root() {
  local env="$1"
  local boot_args=("-var=environment=${env}" "-var=aws_region=${AWS_REGION}")
  if [ "$env" = local ]; then
    boot_args+=("-var=floci_endpoint=${FLOCI_ENDPOINT}")
    export AWS_ENDPOINT_URL_S3="${FLOCI_ENDPOINT}"
  fi
  tf_in "$ROOT/infra/bootstrap" init -input=false -no-color >/dev/null
  tf_in "$ROOT/infra/bootstrap" apply -input=false -auto-approve "${boot_args[@]}"
  tf_backend_init "$env"
}
