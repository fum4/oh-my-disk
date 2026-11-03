# Infrastructure and required checks

## Ownership and prerequisites

The owner chose permanent Terraform-managed required checks for PR #1; there is
no pending choice between a ruleset and a one-time merge exception. Configuration
lives in `infra/terraform/`; it does not take effect until an authorized apply.

An authorized operator performs secure bootstrap:

1. Create or verify the repo-owned private R2 bucket `oh-my-disk-backup`, EU
   jurisdiction, through the devbox `bin/devbox-r2-bucket-create oh-my-disk`
   bootstrap command. Never substitute another repo’s state bucket.
2. Supply the repo age public recipients in `secrets/recipients.txt` and deliver
   the private key to `~/.config/age/oh-my-disk.key` with mode `0600`, with durable
   key storage and devbox escrow per the canonical repo-secrets recipe.
3. Supply the Account R2 credential named `oh-my-disk-terraform-state`, Object
   Read & Write scoped only to `oh-my-disk-backup`. Encrypt its
   `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` as the manifest-declared
   `secrets/oh-my-disk-terraform-state.env.age`. No placeholder values, plaintext
   env files, or new GitHub token. Use the secure editor:

   ```sh
   EDITOR=nano mise run secrets:edit secrets/oh-my-disk-terraform-state.env.age
   ```

   Enter the two `KEY=VALUE` lines in the editor, never chat. The shared engine
   edits in tmpfs and encrypts when the editor closes.
4. Run `mise run secrets:check`. Existing `gh` authentication must be authorized
   to administer `fum4/oh-my-disk`; the `tf` task exports its token only to the
   Terraform process tree.

## State

The S3 backend uses public non-secret configuration in `backend.tf`:

- Bucket: `oh-my-disk-backup` (EU).
- Endpoint: `https://fbc5bc4bfe835ba7165482b7c8f500ee.eu.r2.cloudflarestorage.com`.
- Key: `terraform/oh-my-disk.tfstate`.
- Credentials: the encrypted env row, decrypted only in memory by `repo-secrets exec`.

Never use local-only state or a different repo’s credential. R2 initialization
and plan/apply are operator steps, not part of the static CI job. Serialize
stateful operations: this backend does not configure a distributed state lock.
Do not save binary plans to disk; they may contain sensitive state.

## Commands

`mise run setup` installs the pinned Terraform tool only; existing Go/Wails
build commands and versions remain in `Makefile`. This CLI/desktop repo has no
long-lived dev stack and needs no `process-compose.yaml`.

```sh
mise run tf:check
```

The check uses a separate `.terraform-check` data directory, formats HCL in
check-only mode, initializes with `-backend=false` and
`-lockfile=readonly`, then validates against the provider schema. It downloads
the locked provider if absent but does not contact GitHub or R2. No secrets are
needed. To update the provider deliberately, change the pin, regenerate
`.terraform.lock.hcl` with backend-disabled init, and review its checksums.

After bootstrap and with the owner’s authorization for live changes:

```sh
mise run tf -- init -input=false
mise run tf -- plan -input=false
mise run tf -- apply
```

Review the plan: only `github_repository_ruleset.required_checks` should be
created for the initial bootstrap. There are no bypass actors; PRs require zero
approvals and the four named Actions checks, without strict rechecks. Deletion
and non-fast-forward updates are blocked. `~DEFAULT_BRANCH` follows the existing
`master`; this stack never renames it.

The `infra` CI job always reports on pull requests, without path filters or
state credentials. If job names change, update `required-checks.tf` in the same
PR. After apply, verify the live required checks before merging PR #1
through `wt merge`; configuration alone is not proof that protection is active.
