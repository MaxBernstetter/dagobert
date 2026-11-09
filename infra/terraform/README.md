# Terraform — dagobert (Floci and AWS)

One root, one code path. `var.environment` is the only switch:

| `environment` | Target | Credentials | Guard |
|---|---|---|---|
| `local` (default) | Floci on `:11566` | fixed fake keys, every service endpoint on Floci | cannot reach AWS |
| `prod` | real AWS | default credential chain | refused unless `allow_cloud_deploy=true` |

## What it creates

- `network.tf`: VPC, three private subnets, security groups (`app` may reach `database` on 5432), DB subnet group. No internet gateway, no NAT.
- `database.tf`: Aurora PostgreSQL Serverless v2 cluster and writer (`db.serverless`, 0.5 to 2 ACU by default), `DATABASE_URL` in SSM (SecureString).
- `frontend.tf`: the web app. ECR repository, ECS cluster, Fargate task definition and service (execution role, log group, security group), public subnets with an internet gateway (no NAT). `scripts/infra-apply.sh` creates the repository first, builds and publishes the frontend image (locally Floci runs the image under its ECR URI without a push; in CI it is pushed to ECR), then creates the rest.
- `main.tf`: S3 lake, SSM parameters, task IAM role. `experiment_fanout.tf`: Step Functions map and optional experiment Lambda.

## Apply

```bash
task infra:local          # Floci: starts it, applies, same as `task infra:apply ENV=local`
task infra:prod           # CI only
```

Both run `scripts/infra-apply.sh`, using `terraform` if installed, else the `hashicorp/terraform` image.

## Why a dev setup cannot deploy to the cloud

Three independent checks, all refusing by default:

1. `scripts/infra-apply.sh prod` exits unless `CI=true` and AWS credentials are present.
2. `local` unsets every AWS credential variable before Terraform runs and uses fake keys.
3. `gate.tf` fails any `environment=prod` plan unless `allow_cloud_deploy=true`. Only the script in CI sets `TF_VAR_allow_cloud_deploy`.

## Database

Local: Floci starts a real Postgres container for the cluster and proxies it on port 7001 with TLS, so URLs use `sslmode=require` in both environments. The master password is `POSTGRES_PASSWORD` (default `trading`) locally and generated in prod.

Prod has no load balancer in front of the frontend yet: the service gets a public task IP on `frontend_port`.

Prod Aurora is private to the VPC. Atlas migrations cannot reach it from outside; running them from inside the VPC is not built yet.

## Next layers

API Gateway plus Lambda for the Experiment API (then `frontend_api_url` points at it), the C++ simulation Lambda behind Step Functions. See `docs/grooming/serverless-aws.md`.
