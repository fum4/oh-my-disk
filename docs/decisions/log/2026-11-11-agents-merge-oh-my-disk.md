# Agents merge oh-my-disk behind green required checks

decided: tier=flag reversible=yes veto-by=2026-11-13

oh-my-disk had no factory.toml, so no prior merge tier. `[merge] by = "agents"` in cerebro/repo.toml records today's practice: agents open the PR, the GitHub required checks go green, an agent merges. Every push to master runs the release workflow, so an agent merge is a release. The approval paths (cerebro/**, workflows, infra/terraform/**, secrets/manifest, migrations) still need the owner's go whoever merges.

This is the owner's call: veto it and the line becomes `by = "owner"`, which given merge = release is a fair choice. Alternative dropped: `owner`, which would stop merges the repo already does today.

By: cerebro layout migration (agent), from the hub's brief
