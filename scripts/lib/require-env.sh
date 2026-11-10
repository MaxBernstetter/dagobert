# Sourced by the deploy and dev scripts. The credentials and region come only from the environment:
# `.env` locally (task loads it) and the CI environment in GitHub Actions. Nothing here supplies a value.

# require_env VAR...: exit with a pointer to env.example when any variable is unset or empty.
require_env() {
  local missing=() name
  for name in "$@"; do
    [ -n "${!name:-}" ] || missing+=("$name")
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    echo "Missing environment variables: ${missing[*]}. Set them in .env (copy env.example) or in the CI environment." >&2
    exit 1
  fi
}
