terraform {
  required_version = "= 1.17.1"

  required_providers {
    github = {
      source  = "integrations/github"
      version = "= 6.13.0"
    }
  }
}

provider "github" {
  owner = "fum4"
}
