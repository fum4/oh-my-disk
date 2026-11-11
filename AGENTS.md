# oh-my-disk: repo rules

This file adds only what is specific to oh-my-disk. The cross-repo rules are the owner's global agent rules (devbox `agents/AGENTS.md`).

## What this is
A local disk-space analyzer for developers, with read-only scanning and explicit Move to Trash: a Go CLI for macOS and Linux plus a native Wails macOS desktop app sharing one explorer (README). Public repo: GitHub Actions is its CI.

## Asks and decisions
About to ask the owner anything, just chose between options, or just heard a decision → the `cerebro-ledger` skill: it's a file in the repo (`ledger/asks/`, `docs/decisions/log/`), not a chat message, even if the skill didn't fire.

## Routing
| Occasion | Doc | Guardrail |
|---|---|---|
| Scanning, Trash, the cache or the local server | [README.md](README.md) § Safety, privacy and accounting | Scanning stays read-only; Trash needs an explicit confirmation and never falls back to permanent deletion |
| Build, test, fixtures | [README.md](README.md) § Development and verification | Tests use fixtures or temp dirs, never real user data |
| The desktop app and its agent chat | [README.md](README.md) § Native macOS desktop app | No user MCP configuration is edited |
| Merging, releases | [README.md](README.md) § Releases | Every push to `master` runs the release workflow: a merge is a release |
| Performance work | [PERF.md](PERF.md) | Fixtures only under ignored `.fixtures/`, measurements under `.build/` |
| Terraform, R2 state, required checks | [docs/runbooks/infra.md](docs/runbooks/infra.md) | Never local-only state or another repo's credential; plan/apply are operator steps |
| What's next | [ledger/backlog.md](ledger/backlog.md) | New follow-ups go there, in its heading format |
