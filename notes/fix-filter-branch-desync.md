# Fixing Fork Desync After `git filter-branch` Email Rewrite

**Date:** 2026-04-13

## The Problem

Ran `git filter-branch` to update author email across all commits:

```bash
git filter-branch --env-filter '
OLD_EMAIL="tocruz@nvidia.com"
NEW_EMAIL="cruznyc+nv@gmail.com"

if [ "$GIT_COMMITTER_EMAIL" = "$OLD_EMAIL" ]; then
    export GIT_COMMITTER_EMAIL="$NEW_EMAIL"
fi
if [ "$GIT_AUTHOR_EMAIL" = "$OLD_EMAIL" ]; then
    export GIT_AUTHOR_EMAIL="$NEW_EMAIL"
fi
' --tag-name-filter cat -- --branches --tags
```

This rewrote **every commit** in the repository — not just Tony Cruz's commits, but also commits by Simon Lynch and dependabot. In git, a commit hash is derived from the commit's content plus its parent hash. So even though only Tony Cruz's commits had their email changed, every downstream commit got a new hash because its parent changed. This caused the fork (`origin`: `antoncru/terraform-provider-bcm`) to completely diverge from upstream (`hashi-demo-lab/terraform-provider-bcm`), with 464 commits on each side showing as different despite having identical code.

GitHub detected this divergence and displayed a "Sync fork" button.

## The Fix

### Step 1: Fetch upstream (the untouched original history)

```bash
git fetch upstream
```

Upstream was unaffected by `filter-branch` since it only rewrote the local repo.

### Step 2: Reset `main` to match `upstream/main`

```bash
git checkout main
git reset --hard upstream/main
```

This threw away the rewritten `main` history and replaced it with the original upstream history. Since there were no unique commits on `main` (it was a pure fork), nothing was lost.

**Verification:** `git rev-list --count main...upstream/main` returned `0 0` — zero divergence, same commit hash (`1ac11cd`).

### Step 3: Identify unique commits on the feature branch

```bash
git log --oneline --reverse upstream/main..device-model-and-power-operation --author="Tony Cruz" --format="%H"
```

This found 14 commits that existed only on the feature branch (not in upstream). These commits had the rewritten email (`cruznyc+nv@gmail.com`) which was the desired outcome.

### Step 4: Reset the feature branch and cherry-pick

```bash
git checkout device-model-and-power-operation
git reset --hard upstream/main
git cherry-pick <14 commit hashes>
```

This:
1. Reset the feature branch to the clean upstream base
2. Re-applied the 14 unique commits on top

The cherry-picked commits retained the new email (`cruznyc+nv@gmail.com`) since that's what was in the rewritten commits.

### Step 5: Force-push both branches to origin

```bash
git push --force origin main
git push --force origin device-model-and-power-operation
```

Force-push was required because the remote still had the old (rewritten) history. This replaced it with the fixed history.

### Step 6: Add `notes/` to `.gitignore`

Added `notes/` to `.gitignore` to keep local notes out of the repository.

## What is `git cherry-pick`?

Cherry-pick takes one or more existing commits and replays their changes onto the current branch, creating brand-new commits in the process.

### How it works

Imagine you have two branches with divergent histories:

```
upstream/main:   A --- B --- C --- D
                                    \
feature (rewritten):                 B' --- C' --- D' --- X' --- Y'
```

After `filter-branch`, commits `B'`–`Y'` have different hashes than `B`–`D` even though most of the code is the same. Commits `X'` and `Y'` are your unique work.

Cherry-pick lets you take just `X'` and `Y'` and replay their diffs onto a clean base:

```
upstream/main:   A --- B --- C --- D
                                    \
feature (fixed):                     X'' --- Y''
```

The resulting `X''` and `Y''` have:
- **Same code diffs** as `X'` and `Y'` (your work is preserved)
- **Same author name and email** as `X'` and `Y'` (the new email is kept)
- **New commit hashes** because they have a different parent (`D` instead of `D'`)

### Key properties

- **It copies changes, not commits.** The original commit objects still exist in git's object store (until garbage collected), but the new commits are independent copies with new hashes.
- **It preserves author metadata.** The author name, email, and date from the original commit carry over to the cherry-picked commit. This is why our fix kept the `cruznyc+nv@gmail.com` email.
- **It can fail with conflicts.** If the code being replayed conflicts with the new base, git will pause and ask you to resolve the conflict — just like a rebase. In our case all 14 commits applied cleanly because the upstream base was identical code.
- **It does not move or delete the source commits.** The originals remain on whatever branch (or dangling in the reflog) they came from. Cherry-pick is purely additive.

### Cherry-pick vs. rebase vs. merge

| Operation | What it does | When to use |
|-----------|-------------|-------------|
| `cherry-pick` | Copy specific commits onto current branch | Selectively moving a few commits to a different base |
| `rebase` | Replay all commits from a branch onto a new base | Updating a feature branch to the latest main |
| `merge` | Combine two branches, preserving both histories | Integrating completed work back into main |

In our fix, cherry-pick was the right choice because we needed to move exactly 14 specific commits from the rewritten history onto a clean upstream base, discarding everything else.

## Why It Worked

| Concept | Explanation |
|---------|-------------|
| **Commit hashes are recursive** | A commit's hash depends on its content AND its parent's hash. Changing one early commit cascades new hashes to every descendant. |
| **Upstream was untouched** | `filter-branch` only rewrote the local clone. Upstream still had the original history, serving as the source of truth. |
| **`reset --hard` replaces history** | Points the branch ref to a different commit, discarding the rewritten chain entirely. |
| **Cherry-pick copies diffs, not hashes** | Cherry-pick creates new commits with the same code changes but based on the new (clean) parent. The author metadata (including the new email) is preserved. |
| **Force-push overwrites remote** | Replaces the remote's rewritten history with the fixed local history. |

## Final State

| Branch | Status |
|--------|--------|
| `main` | Identical to `upstream/main` (same hashes) |
| `device-model-and-power-operation` | Clean upstream base + 14 commits with `cruznyc+nv@gmail.com` |
| Upstream commits | Original authors/emails preserved |
| Fork sync | No divergence — "Sync fork" button gone |

## Lesson Learned

`git filter-branch` rewrites ALL commits in the specified range, not just those matching the filter condition. Even commits that don't match the email condition get new hashes because their parents changed. For a forked repo, this makes the fork completely diverge from upstream.

A safer alternative for future email changes on only your own commits: use `git rebase -i` with `exec git commit --amend --author="..."` on just the feature branch, which limits the blast radius to only your unique commits.

---

## Appendix: Anatomy of the Commit Discovery Command

The following command was used in Step 3 to find the commits that needed to be cherry-picked:

```bash
git log --oneline --reverse upstream/main..device-model-and-power-operation --author="Tony Cruz" --format="%H"
```

### Breakdown

**`git log`** — Show commit history.

**`upstream/main..device-model-and-power-operation`** — A commit range filter using double-dot syntax. It means: "show commits reachable from `device-model-and-power-operation` but NOT reachable from `upstream/main`." In other words, only the commits unique to the feature branch — everything added after it diverged from upstream.

```
upstream/main:   A --- B --- C --- D
                                    \
feature:                             X --- Y --- Z
                                     ^-----------^
                                     only these are shown
```

**`--author="Tony Cruz"`** — Further filters to only show commits where the author name matches "Tony Cruz". This excluded any commits from other authors (Simon Lynch, dependabot) that might have ended up on the branch.

**`--oneline`** — Normally displays each commit as a single line (abbreviated hash + subject). In this command it's actually overridden by `--format` (see below), so it has no visible effect. Included out of habit — it's redundant here.

**`--reverse`** — Show commits in chronological order (oldest first) instead of the default reverse-chronological (newest first). This was important because `git cherry-pick` applies commits sequentially — they needed to be in the right order so each commit's parent existed before it was applied.

**`--format="%H"`** — Override the default log output format. `%H` means "full 40-character commit hash." This produced just the raw hashes with no other text, making them easy to copy-paste directly into the `git cherry-pick` command.

### Result

The command produced a chronologically-ordered list of full commit hashes for all 14 unique commits on the feature branch:

```
d5af03ce730b025e4a3c752d8278250b556c85e8
329a67f4b7be810b4e23f42b5504cbbefba5ea6e
9341fa5de4c5448d9bcc7dcff85ae7b36a9cef7a
...
af19c084ab85691d9bc41b50958f9f1798357bc9
```

These were then fed directly into `git cherry-pick` in Step 4.
