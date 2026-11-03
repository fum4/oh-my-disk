# Owner decisions

- [ ] Release/merge blocker: choose a Terraform-managed GitHub required-checks
  ruleset for `master`, or explicitly approve a one-time green-check gate
  exception for PR #1. No rulesets or branch protection currently exist;
  `gh pr checks --required` reports no required checks. Reply “add ruleset” or
  explicitly approve the one-time exception. Infra remains unchanged.
