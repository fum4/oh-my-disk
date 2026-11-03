# Terraform stack

Owns the active `oh-my-disk-required-checks` GitHub repository ruleset. It follows
`~DEFAULT_BRANCH` (currently `master`) without changing the default branch.
Pull requests are mandatory with zero required approvals; deletions and force
pushes are blocked. There are no bypass actors and no strict up-to-date recheck.

Required GitHub Actions checks (app ID `15368`) match `.github/workflows/ci.yml`:

- `test (ubuntu-latest)`
- `test (macos-latest)`
- `desktop`
- `infra`

Terraform `1.17.1` and `integrations/github` `6.13.0` are pinned. The committed
`.terraform.lock.hcl` pins provider checksums. GitHub auth comes from the existing
`gh auth token` only, exported in memory by `mise run tf -- <args>`. Backend
credentials enter that process through the shared `repo-secrets` engine.

Run `mise run tf:check` for formatting, backend-disabled initialization with a
read-only lockfile, and schema validation. This requires provider downloads on
the first run, but no R2 state, GitHub authentication or age key. CI runs this same
task as `infra` on every pull request. Do not rename a CI job without updating
the ruleset contexts here in the same change.

State location and bootstrap procedure: [infrastructure runbook](../../docs/runbooks/infra.md).
Extend this stack with named Terraform resources; never add an imperative GitHub
settings script, credentials to HCL, or local state to git.
