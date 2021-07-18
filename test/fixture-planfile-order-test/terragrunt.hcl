# Configure Terragrunt to automatically store tfstate files in an S3 bucket
remote_state {
  backend = "local"
  config = {}
}

generate "test-null-provider" {
  path      = "test-provider.tf"
  if_exists = "overwrite_terragrunt"

  contents = <<EOF
provider "null" {
}
EOF
}

terraform {
  extra_arguments "plan_vars" {
    commands = [
      "plan",
    ]

    arguments = [
      "-out=${get_terragrunt_dir()}/default.tfplan",
      "-var-file=${get_terragrunt_dir()}/vars/variables.tfvars",
    ]
  }

  extra_arguments "apply_vars" {
    commands = [
      "apply",
    ]

    arguments = [
      "${get_terragrunt_dir()}/default.tfplan",
      "-var-file=${get_terragrunt_dir()}/vars/variables.tfvars",
    ]
  }
}
