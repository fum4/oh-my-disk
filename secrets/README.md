# Encrypted secrets

Data only: the devbox `repo-secrets` engine owns encryption, validation and
in-memory env injection through the root `secrets:*` mise tasks. No copied engine
or per-repo wrapper belongs here.

| Credential | Encrypted source | Keys | Consumer / scope |
| --- | --- | --- | --- |
| `oh-my-disk-terraform-state` | `oh-my-disk-terraform-state.env.age` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | Terraform S3 backend; Account R2 Object Read & Write on `oh-my-disk-backup` only |

`manifest` declares an `env` row: the secret is never materialized to disk.
`recipients.txt` contains the repo public age recipient. Operators bootstrap
the encrypted credential and private repo key
at `~/.config/age/oh-my-disk.key` (mode `0600`), including its durable escrow.
Until those exist, `secrets:check` and stateful Terraform commands must fail;
do not fill the gap with placeholder credentials or an empty recipient file.

`mise run secrets:edit secrets/oh-my-disk-terraform-state.env.age` rotates the
encrypted env bundle in tmpfs. Run `mise run secrets:check` after bootstrap or
rotation. Keep plaintext out of this folder. GitHub provider access reuses
`gh auth token` in memory, not another stored GitHub token.

See the [infrastructure runbook](../docs/runbooks/infra.md) for state details and
bootstrap ownership. Add each new secret as a manifest row and inventory entry
before wiring a consumer.
