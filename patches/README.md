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
patch -p1 --dry-run < patches/<NUMBER>-<short-slug>.patch

# 3. rebuild — the build applies it automatically
docker compose -f docker-compose.yml -f docker-compose.prod-local.yml up -d --build new-api
```

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

## When a PR has several commits

`patch -p1` walks a `.patch` commit by commit, so a PR whose later commits edit
lines its earlier commits just wrote can fail even though the end state applies
fine. Download `pull/<NUMBER>.diff` instead — the squashed diff — and save it
under the same `<PR-number>-<short-slug>.patch` name. That is what
`6355-claude-cache-creation-split.patch` is.

## Reconciled patches

`6629-deepseek-v4-stream-edges.patch` is **not** a plain download. PR #6629 and
PR #6394 both rewrite the OpenAI→Claude stream lifecycle, so #6629 was
re-anchored on top of #6394 and regenerated; its commit message records what
changed and how to redo it. Re-download it and you get the two rejects back.

Acceptance for that pair is both PRs' tests green together:

```bash
cd relaykit && GOWORK=off go test -count=1 ./relayconvert/internal/oai_chat/
```

## Known quirks

- `6302-openai-claude-cached-tokens.patch` applies with fuzz, not an exact
  match, and it deliberately breaks an upstream golden test:
  `relaykit/relayconvert` asserts `input_tokens: 10` where the fix yields `7`.
  The image build does not run tests, so this only shows up in a local
  `go test ./relayconvert/...`. Upstream's PR head also lives under
  `service/relayconvert/` paths — re-downloading it means rewriting them to
  `relaykit/`.
