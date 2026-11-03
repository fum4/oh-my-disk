locals {
  github_actions_app_id = 15368
  required_checks = toset([
    "test (ubuntu-latest)",
    "test (macos-latest)",
    "desktop",
    "infra",
  ])
}

resource "github_repository_ruleset" "required_checks" {
  name        = "oh-my-disk-required-checks"
  repository  = "oh-my-disk"
  target      = "branch"
  enforcement = "active"

  conditions {
    ref_name {
      include = ["~DEFAULT_BRANCH"]
      exclude = []
    }
  }

  rules {
    deletion         = true
    non_fast_forward = true

    pull_request {
      required_approving_review_count = 0
    }

    required_status_checks {
      strict_required_status_checks_policy = false

      dynamic "required_check" {
        for_each = local.required_checks
        content {
          context        = required_check.value
          integration_id = local.github_actions_app_id
        }
      }
    }
  }
}
