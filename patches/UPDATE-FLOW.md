# Updating `production` from upstream

How to pull new changes from `QuantumNous/new-api` (upstream `main`) into our
`production` deploy branch. We use **merge** (not rebase) — simpler, preserves
history, and we update frequently enough that conflicts stay small.

Run this whenever upstream ships a release you want.

## Prerequisites (one time)

Remotes are already set up:
- `origin` → `QuantumNous/new-api` (upstream)
- `fork`   → `neimaravila/new-api` (where `production` lives)

## The update (normal case)

```bash
cd /path/to/new-api
git checkout production
git pull fork production            # make sure local is current

# 1. fetch latest upstream
git fetch origin

# 2. merge upstream main into production
git merge origin/main
```

Most of the time this is a clean fast-forward or auto-merge. If it conflicts,
see **Conflict hotspots** below.

```bash
# 3. are any patches now merged upstream? check + clean up
#    (the build will FAIL if a stale patch remains, so this is mandatory)
ls patches/*.patch
# for each patch: if its PR is now in origin/main, delete it:
#   git rm patches/<N>-<slug>.patch

# 4. verify the image still builds (applies remaining patches + compiles)
docker compose -f docker-compose.yml -f docker-compose.prod-local.yml build new-api

# 5. if green, push
git push fork production
```

Production rebuilds from `fork/production` and you're done.

## Conflict hotspots

Two files are likely to conflict because both we and upstream touch them:

### 1. `Dockerfile` (most likely)

We added the patch step before `go build`. Upstream also edits this file.

Resolution — keep BOTH our patch line and upstream's changes. The shape we
want preserved:

```dockerfile
COPY . .
COPY --from=builder /build/web/dist ./web/dist
RUN apk add --no-cache patch && sh scripts/apply-patches.sh    # ← keep this line
RUN go build ...                                                # ← keep upstream's go build
```

Accept upstream's other changes (base image pins, new build flags), just make
sure the `apply-patches.sh` line stays right before the final `go build`.

### 2. `i18n/i18n.go` and `web/src/i18n/config.ts`

We force pt-BR as default (`DefaultLang = LangPtBR`, `fallbackLng: 'ptBR'`,
detection by `localStorage` only). Upstream may refactor i18n init.

Resolution — keep our defaults; if upstream changed surrounding code, port
our three values into the new shape:
- `i18n.go`: `DefaultLang = LangPtBR`
- `config.ts`: `fallbackLng: 'ptBR'`, `order: ['localStorage']`

### 3. Translation files (`web/src/i18n/locales/pt-BR.json`, `i18n/locales/pt-BR.yaml`)

Unlikely to conflict (upstream doesn't touch our pt-BR files). If the public
pt-BR PR gets merged upstream, these may overlap — in that case prefer the
upstream version and drop our copy.

## If a patch no longer applies

After merge, a patch we carry might be **already merged upstream**. The build
will fail with:

```
[apply-patches] WARNING: 6394-...patch does not apply cleanly.
```

That's the intended signal. Fix:

```bash
git rm patches/<N>-<slug>.patch
git commit -m "build: drop patch <N> (merged upstream)"
# rebuild — should be green now
```

## Quick health check after update

```bash
# 1. image builds?
docker compose -f docker-compose.yml -f docker-compose.prod-local.yml build new-api

# 2. starts and i18n still has pt-BR?
docker compose -f docker-compose.yml -f docker-compose.prod-local.yml up -d
docker logs new-api 2>&1 | grep "i18n initialized"
# expect: i18n initialized with languages: zh-CN, zh-TW, en, pt-BR
```

If all green, `git push fork production` and the deploy updates.
