# Terraform — dagobert (Floci and AWS)

One root, one code path. `var.environment` is the only switch:

| `environment` | Target | Credentials | Guard |
|---|---|---|---|
| `local` (default) | Floci on `:11566` | fixed fake keys, every service endpoint on Floci | cannot reach AWS |
| `prod` | real AWS | default credential chain | refused unless `allow_cloud_deploy=true` |

## What it creates

- `network.tf`: VPC, three private subnets (database, Lambda), three public subnets with an internet gateway (ECS data-prep tasks only, public IP, no NAT), security groups (`app` may reach `database` on 5432, S3 on 443 and the `vpc_endpoints` group on 443; `jobs` may reach `database` on 5432, egress 443 and 5432 only, no ingress), DB subnet group, private route table with an S3 gateway endpoint. No NAT, no interface endpoints.
- `db_migrate.tf`: the one-off migration task. ECR `<prefix>-db-migrate`, ECS cluster `<prefix>-ops`, Fargate task definition (256 CPU, 512 MB, `awsvpc`, private subnets, `app` security group, no public IP), execution role (pulls the image, reads `DATABASE_URL` from SSM via task `secrets`), empty task role, log group `/ecs/<prefix>-db-migrate`. Prod only: interface endpoints `ecr.api`, `ecr.dkr`, `logs`, `ssm` in one private subnet (about 4 x 0.01 USD per hour; the only idle cost in prod). Floci needs none.
- `api_lambda.tf`: the Experiment API as a container-image Lambda (ECR `<prefix>-experiment-api`, VPC-attached to the private subnets with the `app` security group, dedicated role, 29 s, 512 MB), behind an API Gateway HTTP API. Route `POST /tradingcopilot.v1.ExperimentService/{method}`, `$default` stage, CORS from `api_cors_allow_origins`. The CloudFront distribution in `frontend.tf` routes `/tradingcopilot.v1.*` here.
- `database.tf`: Aurora PostgreSQL Serverless v2 cluster and writer (`db.serverless`, 0.5 to 2 ACU by default), `DATABASE_URL` in SSM (SecureString).
- `frontend.tf`: the web app. A private S3 bucket (`<prefix>-frontend`, public access blocked) and a CloudFront distribution (price class 100, default certificate). One origin per path: the default behavior serves the SPA from S3 (cached; OAC in prod), `/tradingcopilot.v1.*` goes to the API Gateway (no cache, all methods, viewer headers except Host). A viewer-request CloudFront function (default behavior only) rewrites extensionless paths to `/index.html` (BrowserRouter fallback). No error mapping, so API 403/404 stay as Connect returns them. Same origin for the browser, so the API needs no CORS for this site. `scripts/frontend-deploy.sh` builds the SPA and uploads it after each apply; Terraform does not upload files.
- `jobs_ecs.tf`: the data-prep jobs as ECS Fargate tasks. ECR `<prefix>-ingest` and `<prefix>-indicator`, cluster `<prefix>-jobs`, task definitions (ingest 0.5 vCPU / 1 GB, indicator 1 vCPU / 2 GB), log groups `/ecs/<prefix>-<job>` (14 days), execution role (ECR pull, logs, SSM read for secrets). Task role: the shared `aws_iam_role.task` (S3 lake, SSM read). Ingest gets `DATABASE_URL` and `BINANCE_BASE_URL` as task `secrets` from SSM (never plain env). Env: `LAKE_BACKEND=s3`, `LAKE_BUCKET`, `DATA_DIR=/tmp/dagobert-data` (scratch only). Exit codes unchanged (0, 1, 2).
- `backfill_sfn.tf`: Step Functions `<prefix>-backfill`. Ingest, then indicator, each `ecs:runTask.sync`. The containers set `COMPOSE_OK_ON_ABORT=1` (`jobs_ecs.tf`), so exit 2 (already stored) exits 0 and the task succeeds; exit 1 or a task that does not start ends the execution FAILED. Input `{provider, base, quote, from, to}`. Output: `backfill_sfn_arn`.
- `main.tf`: S3 lake, SSM parameters, task IAM role. `experiment_fanout.tf`: Step Functions map and optional experiment Lambda.

Outputs used by the scripts: `frontend_bucket`, `frontend_distribution_id`, `frontend_domain`, `experiment_api_repository_url`, `api_gateway_url`, `backfill_sfn_arn`, `ingest_repository_url`, `indicator_repository_url`, `jobs_cluster`, and for the migration task `db_migrate_repository_url`, `db_migrate_cluster`, `db_migrate_task_family`, `db_migrate_subnet_ids`, `db_migrate_security_group_id`, `db_migrate_log_group`.

## Apply

```bash
task infra:local          # Floci: starts it, applies, same as `task infra:apply ENV=local`
task infra:prod           # CI only
```

Both run `scripts/infra-apply.sh`, using `terraform` if installed, else the `hashicorp/terraform` image. The script applies Terraform for the API image, builds and publishes it, creates the job repositories, builds and publishes the ingest and indicator images (`JOB_IMAGE_TAG`, default `latest`), applies the rest, and then runs `scripts/frontend-deploy.sh <env>`.

## Backfill

`scripts/backfill-start.sh <local|prod> '<json>'` starts one `<prefix>-backfill` execution and waits for it. Input: `{"provider":"binance","base":"BTC","quote":"USDT","from":<ms>,"to":<ms>}`. It exits 0 when the execution succeeded. Each run covers one market; the orchestrator's multi-symbol loop is not ported. See `docs/architecture/local-stack.md` for local use.

To redeploy only the web app (after the bucket exists): `scripts/frontend-deploy.sh local` or `scripts/frontend-deploy.sh prod` (prod needs `CI=true`). The script builds `src/frontend` with `VITE_EXPERIMENT_API_URL` empty, syncs `dist/assets/` with `Cache-Control: public,max-age=31536000,immutable`, the other root files with `no-cache`, and `index.html` last with `no-cache`. Prod then invalidates `/index.html`. Old hashed assets are not deleted.

## Why a dev setup cannot deploy to the cloud

Three independent checks, all refusing by default:

1. `scripts/infra-apply.sh prod` exits unless `CI=true` and AWS credentials are present.
2. `local` unsets every AWS credential variable before Terraform runs and uses fake keys.
3. `gate.tf` fails any `environment=prod` plan unless `allow_cloud_deploy=true`. Only the script in CI sets `TF_VAR_allow_cloud_deploy`.

## Database

Local: Floci starts a real Postgres container for the cluster and proxies it on port 7001 with TLS, so URLs use `sslmode=require` in both environments. The master password is `POSTGRES_PASSWORD` (default `trading`) locally and generated in prod.

Prod web app: CloudFront default domain only (`frontend_domain` output). Custom domain and ACM certificate are a follow-up.

Prod Aurora is private to the VPC. Migrations run there as a one-off ECS task: `task db:migrate:ecs` (local, after `task infra:local`) or `scripts/db-migrate.sh prod` (CI, after `scripts/infra-apply.sh prod`). The script builds `docker/db-migrate/Dockerfile`, pushes it in prod, runs the task with `aws ecs run-task`, waits with `aws ecs wait tasks-stopped`, prints the CloudWatch log and fails unless the container exited 0 (a missing exit code also fails). The CI workflow that calls it is not in this repository yet.

## Web app (CloudFront)

- Local: CloudFront and S3 are Floci's own endpoints, reached as `127.0.0.1:4566` inside Floci (the host port `11566` is not the internal port). The S3 origin is a custom origin with path `/<bucket>`, the API origin `/_aws/execute-api/<api-id>/$default`. Floci needs `FLOCI_SERVICES_CLOUDFRONT_ALLOWED_PRIVATE_ORIGIN_HOSTS=127.0.0.1` (set in `compose.yaml`).
- Prod: S3 origin via OAC with a bucket policy for this distribution only; API origin is the API Gateway host over https.
- The local site: `curl -H "Host: $(terraform output -raw frontend_domain)" http://127.0.0.1:11566/` (Floci routes distributions by Host). Verified on `floci/floci:2.4.0`: `/` and `/assets/*` are served, and the Connect POST works through the distribution. Not verified: SPA deep links. Floci accepts `aws_cloudfront_function` (status DEPLOYED, associated) but does not run it, so `/some/route` returns S3's `NoSuchKey` 404 locally. Prod relies on the function; use `task frontend:dev` for deep links.
- Known gaps: no custom domain or TLS certificate, no authentication, Floci does not enforce OAC, the 6 MB API Gateway payload limit applies, and Floci does not execute CloudFront functions (SPA deep links only work in prod).
- Local dev UI stays `task frontend:dev` (Vite on `:3000`, proxies `/api` and `/tradingcopilot.v1.` to the orchestrator). The deployed site does not route `/api/*` (the provider REST routes): the Connectors page works only in the dev server.

## Experiment API gateway

- Call: `curl -X POST "$(terraform output -raw api_gateway_url)/tradingcopilot.v1.ExperimentService/<Method>" -H 'content-type: application/json' -d '{}'`. On Floci the output host does not resolve; call `http://127.0.0.1:11566/_aws/execute-api/<api-id>/\$default/tradingcopilot.v1.ExperimentService/<Method>` instead (api id = first label of the output host); the browser reaches it through CloudFront (see Web app).
- Image: `scripts/infra-apply.sh` builds `docker/experiment-api/Dockerfile` under the ECR URI (`API_IMAGE_TAG`, default `latest`); prod pushes it first, because Lambda needs the image to exist at create time.
- Known gaps: no auth; API Gateway payload limit (about 6 MB per request and response); no RDS Proxy, so every cold start opens its own pool; Run/Cancel hooks still write the database only; SSM and other AWS APIs are not reachable from the VPC Lambda (no NAT, no interface endpoints), only Aurora and S3.

## Next layers

The C++ simulation Lambda behind Step Functions. See `docs/grooming/serverless-aws.md`.
