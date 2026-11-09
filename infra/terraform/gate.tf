# Cloud-deploy gate. environment=local only ever talks to Floci (providers.tf).
# environment=prod touches real AWS, so it needs allow_cloud_deploy, which only CI sets.
resource "terraform_data" "deploy_gate" {
  lifecycle {
    precondition {
      condition     = var.environment == "local" || var.allow_cloud_deploy
      error_message = "Refusing to deploy to real AWS: environment=prod requires allow_cloud_deploy=true, which only CI sets (task infra:apply does this when CI=true)."
    }
  }
}
