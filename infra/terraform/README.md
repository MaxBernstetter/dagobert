# Terraform — dagobert (Floci + AWS)

Creates the **$0-at-idle** data plane: S3 lake, SSM parameters, and a shared task IAM role. The experiment catalog lives in Postgres, not DynamoDB.

No NAT, ALB, RDS, Secrets Manager, or always-on compute.

## Local (Floci)

```bash
# From repo root — Floci already in compose, or:
#   floci start && eval $(floci env)

docker compose up -d floci

cd infra/terraform
terraform init
terraform apply -var='environment=local' -var='aws_region=us-east-1'
```

Verify:

```bash
export AWS_ENDPOINT_URL=http://localhost:11566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1

aws s3 ls
aws ssm get-parameters-by-path --path /dagobert/local
```

## Prod (real AWS)

```bash
cd infra/terraform
terraform apply -var='environment=prod' -var='aws_region=eu-central-1'
```

Use an IAM/SSO profile with rights to create S3, IAM, SSM. Do **not** set `AWS_ENDPOINT_URL`.

Prod also creates one Supabase project when `supabase_organization_id` is set, and stores the direct Postgres URL in SSM at `/{project}/{environment}/DATABASE_URL`. Put `SUPABASE_ACCESS_TOKEN` and `SUPABASE_DB_PWD` in the repo `.env`. `task infra:prod` maps those to the Terraform variables. Apply the Atlas migrations in `db/migrations` to that URL. Local applies do not create a Supabase project.

## Next layers (not in this root yet)

- Step Functions state machine + EventBridge Scheduler (3h)
- Lambda / Fargate task definitions for job images
- HTTP API Gateway

See `docs/grooming/serverless-aws.md`.
