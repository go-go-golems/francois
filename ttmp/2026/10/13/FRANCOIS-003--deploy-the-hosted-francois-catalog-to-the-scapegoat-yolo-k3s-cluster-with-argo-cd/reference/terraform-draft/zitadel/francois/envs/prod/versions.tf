terraform {
  required_version = ">= 1.12.0"

  backend "s3" {
    bucket         = "go-go-golems-tf-state"
    key            = "zitadel/francois/prod/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "go-go-golems-tf-state-lock-table"
    encrypt        = true
  }

  required_providers {
    vault = {
      source  = "hashicorp/vault"
      version = "5.10.1"
    }

    zitadel = {
      source  = "zitadel/zitadel"
      version = "3.3.0"
    }
  }
}
