# Upstream PR patches

This directory holds upstream PRs we want **before they're merged**. The build
(`scripts/apply-patches.sh`, wired into the Dockerfile) applies every
`*.patch` here over a clean new-api checkout before compiling.

## Why this exists

Keeps our image = **upstream + thin patch layer**, not a fork. We never edit
upstream files directly for an upstream fix; we carry the PR as a patch so the
history stays clean and updates stay painless.

## Adding a PR we want early

```bash
# 1. download the PR as a patch
curl -sL https://github.com/QuantumNous/new-api/pull/<NUMBER>.patch \
  -o patches/<NUMBER>-<short-slug>.patch

# 2. verify it applies cleanly on our current code
patch -p1 --dry-run --force < patches/<NUMBER>-<short-slug>.patch

# 3. rebuild — the build applies it automatically
docker compose -f docker-compose.yml -f docker-compose.prod-local.yml up -d --build new-api
```

## Verifying locally on macOS

Pass `--force` when you dry-run by hand. Apple's `patch` (BSD 2.0-12u11) detects
an already-applied patch, prints `Reversed (or previously applied) patch
detected!  Assume -R? [y]`, and **exits 0** — so a patch that is already merged
upstream looks like it applied cleanly. `--force` turns that into the honest
failure. `git apply --check` is stricter still, but it rejects the patches we
carry that only apply with fuzz, so it is a second opinion, not the gate.

The real gate is the image build: Alpine's GNU `patch` exits non-zero on a
failed hunk, which is what `apply-patches.sh` relies on.

## Removing a patch after it's merged upstream

**Just delete the file and rebuild.** No revert, no merge conflict — once the
PR is merged, upstream already contains the change, so the patch becomes a
no-op (the build will tell you it doesn't apply cleanly, which is your signal
to delete it).

```bash
rm patches/<NUMBER>-<short-slug>.patch
# rebuild
```

The `apply-patches.sh` script fails the build if a patch doesn't apply, so a
stale (already-merged) patch can't silently break things.

## Conventions

- Name: `<PR-number>-<short-slug>.patch` (e.g. `6394-claude-parallel-tool-block.patch`).
- One PR per file.
- Prefer small, focused, `mergeable` PRs — large refactors are painful to carry.

## Markdown hunks must be stripped

`.dockerignore` excludes `*.md` from the build context, so
`scripts/apply-patches.sh` runs against a tree with no `README.md`. A patch
carrying a `README.md` hunk applies fine in a local dry-run and then fails the
image build — the one failure mode the local check cannot see.

Before adding a patch, check it:

```bash
grep '^diff --git a/.*\.md ' patches/<N>-<slug>.patch
```

If it hits, delete those hunks and say so in the patch header.
`6949-upstream-header-timeout.patch` is the one that needed it. `.env.example`
is *not* excluded, so those hunks stay.

## When a PR has several commits

`patch -p1` walks a `.patch` commit by commit, so a PR whose later commits edit
lines its earlier commits just wrote can fail even though the end state applies
fine. Download `pull/<NUMBER>.diff` instead — the squashed diff — and save it
under the same `<PR-number>-<short-slug>.patch` name. That is what
`6355-claude-cache-creation-split.patch`, `6949-upstream-header-timeout.patch`
and `6982-sse-comment-heartbeats.patch` are — each failed as a multi-commit
`.patch` and applies cleanly as the squashed `.diff`.

## Reconciled patches

Three patches are **not** plain downloads anymore. After the rc.45 upgrade
(2026-10) the upstream rewrote the code they touch, so each was re-implemented
on the new code and regenerated from that state:

- `6394-claude-parallel-tool-block.patch` carries the code for both PR #6394
  (no ghost `content_block_stop` for parallel tool calls) and PR #6629
  (discard text-only frames after tool_use; empty text block when a stream
  emitted none — `HasContentBlock`). Both rewrite the OpenAI→Claude stream
  lifecycle, so they live together; `6629-deepseek-v4-stream-edges.patch`
  holds only #6629's regression tests and applies after #6394's patch.
- `7033-sse-direct-forward.patch` merges PR #7033 (direct frame forwarding
  with a one-frame hold for usage-only frames) with PR #6070 (recover usage
  from the last frame that carried it when non-standard trailing frames
  overwrite it) and the small finalize bridge of #6721 — all three edit the
  same handler, so they ship as one patch. #6721's standalone patch is gone.

Acceptance for the OpenAI→Claude pair:

```bash
cd relaykit && GOWORK=off go test -count=1 ./relayconvert/internal/oai_chat/
```

And for the OpenAI stream handler:

```bash
go test -count=1 ./relay/channel/openai/
```

`6675-rate-limit-group-write-lock.patch` and `6575-oauth-redirect-uri-callback.patch`
were also regenerated on the rc.45 code; `6355-claude-cache-creation-split.patch`
now patches `service/tiered_settle.go` (the old anchor moved into relaykit).

## Order matters

`apply-patches.sh` applies files in shell glob (alphabetical) order. The
patches are name-independent at build time, but two of them touch the same
files: `7033` must apply before the finalize changes it embeds; keep the
current set working by re-running the acceptance tests above after any
re-download.

