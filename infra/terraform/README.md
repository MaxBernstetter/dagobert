# Terraform — dagobert (Floci and AWS)

One root, one code path. `var.environment` is the only switch:

| `environment` | Target | Credentials | Guard |
|---|---|---|---|
| `local` (default) | Floci on `:11566` | `AWS_*` variables from `.env` (dummy values), every service endpoint on Floci | cannot reach AWS |
| `prod` | real AWS | `AWS_*` variables from the CI environment (same names) | refused unless `allow_cloud_deploy=true` |

## State

State lives in S3 with native locking (`use_lockfile`, Terraform 1.10+), in both environments.

- `infra/bootstrap` (its own local state, gitignored) creates the state bucket `<prefix>-tfstate-<account>`: versioned, encrypted, public access blocked, plain HTTP refused.
- `scripts/lib/terraform.sh` (`tf_init_root`) applies the bootstrap, then initialises `infra/terraform` with bucket and key `dagobert/<env>/terraform.tfstate`. Every script that reads or changes state goes through it.
- Local: the same backend, pointed at Floci. A Floci stack created before this backend has its state in a local file; reset it once with `docker compose rm -sf floci && docker compose up -d floci`, then `task up`.

## What it creates

- `network.tf`: VPC, three private subnets (database, Lambda), three public subnets with an internet gateway (ECS data-prep tasks only, public IP, no NAT), security groups (`app` may reach `database` on 5432, S3 on 443 and the `vpc_endpoints` group on 443; `jobs` may reach `database` on 5432 and egress 443 and 5432 only, no ingress), DB subnet group, private route table with an S3 gateway endpoint. No NAT, no interface endpoints.
- `db_migrate.tf`: the one-off migration task. ECR `<prefix>-db-migrate`, ECS cluster `<prefix>-ops`, Fargate task definition (256 CPU, 512 MB, `awsvpc`, private subnets, `app` security group, no public IP). It reads the master URL `MIGRATION_DATABASE_URL` and `APP_DATABASE_PASSWORD` from SSM, and nothing else. Prod only: interface endpoints `ecr.api`, `ecr.dkr`, `logs`, `ssm`, `secretsmanager`, `states` in one private subnet (about 5 x 0.01 USD per hour; the only idle cost in prod).
- `api_lambda.tf`: the Experiment API as a container-image Lambda (ECR `<prefix>-experiment-api`, VPC-attached, dedicated role, 29 s, 512 MB), behind an API Gateway HTTP API. Routes `POST /tradingcopilot.v1.{ExperimentService,ProviderService,BackfillService}/{method}`, `$default` stage, no CORS. Sign-in is in the Lambda itself (`src/internal/auth`): `POST /auth/login` and `/auth/refresh` exchange a user name and password for Cognito tokens, and every other route answers 401 unless the call carries a bearer access token that Cognito's `GetUser` accepts (cached for 60 s). API Gateway runs no authorizer, because Floci does not validate JWTs there; this keeps Floci and AWS on one code path.
- `database.tf`: Aurora PostgreSQL Serverless v2 cluster and writer (`db.serverless`, 0 to 2 ACU, auto-pause after 300 s without connections, so compute costs nothing while idle; the first request after a pause waits about 20 to 30 s for resume, see `src/internal/pgpool`). Two roles: the master user (migrations only) and `dagobert_app` (DML on `public`, nothing else). SSM SecureStrings: `DATABASE_URL` (the app role; read by the jobs and the Lambdas), `MIGRATION_DATABASE_URL` (master), `APP_DATABASE_PASSWORD`.
- `frontend.tf`: the web app. A private S3 bucket (`<prefix>-frontend`, public access blocked, plain HTTP refused) and a CloudFront distribution (price class 100, default certificate). The default behavior serves the SPA from S3 (OAC in prod). `/tradingcopilot.v1.*` and `/auth/*` go to the API. A viewer-request CloudFront function rewrites extensionless paths to `/index.html`.
- `transport.tf`: the lake bucket refuses plain HTTP.
- `jobs_ecs.tf`: the data-prep jobs as ECS Fargate tasks. ECR `<prefix>-ingest` and `<prefix>-indicator`, cluster `<prefix>-jobs`, log groups `/ecs/<prefix>-<job>` (14 days). Ingest gets `DATABASE_URL` and `BINANCE_BASE_URL` as task `secrets` from SSM (never plain env). The shared task role reads only those two parameters and deletes objects only under `experiments/`.
- `backfill_sfn.tf`: Step Functions `<prefix>-backfill`. Ingest, then indicator, each `ecs:runTask.sync`. Exit 2 (already stored) is success.
- `experiment_workflow.tf`: the experiment-worker as a container-image Lambda, the Step Functions state machine `<prefix>-experiment-run`, and its IAM (lake reads and writes; deletes under `experiments/` only).
- `experiment_fanout.tf`: the workflow definition (Prepare -> Map over the simulate batches -> Finalize; any failure runs MarkFailed) and its tunables (`experiment_batch_size`, `experiment_map_max_concurrency`).
- `main.tf`: S3 lake, SSM parameters, task IAM role.

Outputs used by the scripts: `frontend_bucket`, `frontend_distribution_id`, `frontend_domain`, `api_gateway_url`, the ECR repository URLs, and the `db_migrate_*` values for the migration task.

## Apply

```bash
task infra:local          # Floci: state bucket, apply, images, web app upload, same as `task infra:apply ENV=local`
task infra:prod           # CI only
```

Both run `scripts/infra-apply.sh`, which initialises the state (above), applies Terraform for the API image, builds and publishes it, creates the job repositories, builds and publishes the job images, applies the rest, and then runs `scripts/frontend-deploy.sh <env>`. Terraform is the binary if installed, else `hashicorp/terraform:1.10.5`.

## Backfill

Backfills are started through `BackfillService` (`BACKFILL_SFN_ARN`, `api_lambda.tf`). It starts one `<prefix>-backfill` execution per market. `scripts/backfill-start.sh <local|prod> '<json>'` starts one execution directly and waits for it.

To redeploy only the web app: `scripts/frontend-deploy.sh local` or `scripts/frontend-deploy.sh prod` (prod needs `CI=true`). Prod invalidates `/index.html` after the upload.

## Why a dev setup cannot deploy to the cloud

Three independent checks, all refusing by default:

1. `scripts/infra-apply.sh prod` exits unless `CI=true` and AWS credentials are present.
2. Local runs only against Floci endpoints; the scripts unset profile and role variables so a dev shell cannot route to AWS.
3. `gate.tf` (and the bootstrap's own gate) fails any `environment=prod` plan unless `allow_cloud_deploy=true`. Only the script in CI sets `TF_VAR_allow_cloud_deploy`.

## Database

- Local: Floci runs the Aurora cluster and proxies it on port 7001 with TLS. Master password `POSTGRES_PASSWORD` (default `trading`); application role password `POSTGRES_APP_PASSWORD` (default `dagobert_app`).
- Prod: both passwords are generated.
- Migrations run as a one-off ECS task inside the VPC (`task db:migrate:ecs` locally, `scripts/db-migrate.sh prod` in CI). The entrypoint applies `db/migrations` as the master user, then runs `db/roles.sql`, which creates or updates `dagobert_app` and grants DML on `public` (including future tables).
- The application connects as `dagobert_app`. It cannot create or alter tables.
- Residual: the application URL, with the app password, is an environment variable of the Lambdas. Reading it from SSM at start-up needs a code change; it is a follow-up.

## Web app (CloudFront)

- Local: CloudFront and S3 are Floci's own endpoints, reached as `127.0.0.1:4566` inside Floci (the host port `11566` is not the internal port). The S3 origin is a custom origin with path `/<bucket>`, the API origin `/_aws/execute-api/<api-id>/$default`. Floci needs `FLOCI_SERVICES_CLOUDFRONT_ALLOWED_PRIVATE_ORIGIN_HOSTS=127.0.0.1` (set in `compose.yaml`).
- Prod: S3 origin via OAC with a bucket policy for this distribution only. The API origin is the API Gateway host over https.
- The local site: `curl -H "Host: $(scripts/tf-output.sh frontend_domain)" http://127.0.0.1:11566/`. Verified on `floci/floci:2.4.0`: `/` and `/assets/*` are served. Not verified: SPA deep links (Floci does not run CloudFront functions).
- Known gaps: no custom domain or TLS certificate (default certificate, so the minimum TLS version is the CloudFront default), one shared Cognito user pool with admin-created users only (no self sign-up, no roles), Floci does not enforce OAC.

## Experiment API

- Call through CloudFront: `POST https://<frontend_domain>/tradingcopilot.v1.ExperimentService/<Method>` with JSON.
- Authentication: `POST /auth/login` with `{"username","password"}` returns `accessToken`, `refreshToken` and `expiresIn`; send `Authorization: Bearer <accessToken>` on every RPC. Local seeds the user `admin` / `password` (`cognito.tf`; Cognito requires 6+ characters, so `admin` is not possible). Prod has no seeded user: create one with `aws cognito-idp admin-create-user` and `admin-set-user-password --permanent`. The prod Lambda reaches Cognito through the `cognito-idp` interface endpoint (`db_migrate.tf`).
- Image: `scripts/infra-apply.sh` builds `docker/experiment-api/Dockerfile` under the ECR URI (`API_IMAGE_TAG`, default `latest`); prod pushes it first. The worker image is built and pushed the same way.
- Run, Cancel and progress start, stop and describe executions of `<prefix>-experiment-run` (`EXPERIMENT_SFN_ARN`).
- Known gaps: API Gateway payload limit (about 6 MB per request and response); no RDS Proxy, so every cold start opens its own pool; SSM and other AWS APIs are not reachable from the VPC Lambda (no NAT), only Aurora, S3, and in prod Secrets Manager and SSM through interface endpoints (`db_migrate.tf`). Prepare ingests missing candle hours from Binance when coverage is incomplete, which the VPC cannot do, so a symbol without complete coverage fails its prepare.

## Accepted risks

- Ingest runs with a public IP and egress on 443 to any address. Security groups cannot filter by hostname, and a NAT gateway or proxy costs more than the benefit at this scale. Accepted, documented.
- The API Gateway URL is public in prod (CloudFront is not enforced as the only caller). The Lambda refuses unauthenticated calls, so the exposure is the 401 path and the Lambda invocation it costs.
