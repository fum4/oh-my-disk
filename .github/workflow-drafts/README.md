# Activate on the devbox before merging PR #1

These reviewed workflow sources are staged here because the current Mac's fum4
GitHub token has repo scope but not workflow scope. They are not executable here.

With a fum4 credential that has workflow scope:

```sh
cp .github/workflow-drafts/ci.yml .github/workflows/ci.yml
cp .github/workflow-drafts/release.yml .github/workflows/release.yml
git rm -r .github/workflow-drafts
# commit and push to the existing v1 branch / PR #1
```

Activate before merging, and provision the three publishing secrets first. The
release workflow runs on pushes to master, not PRs; it must be present in the
merge commit to publish the initial release. Its publisher pin is the exact head
of https://github.com/fum4/releases/pull/11; merge that PR first.
