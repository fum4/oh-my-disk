# Infrastructure

This folder owns oh-my-disk infrastructure, currently the GitHub default-branch
merge ruleset in [terraform/](terraform/README.md). Terraform uses the repo-owned
EU R2 state bucket; it does not own the bucket bootstrap or mint credentials.

Run infrastructure commands through the root `.mise.toml` tasks. See the
[infrastructure runbook](../docs/runbooks/infra.md) for prerequisites and changes.
Keep new infrastructure in this stack rather than adding a parallel wrapper.
