#!/usr/bin/env sh
# Apply upstream PR patches over a clean new-api checkout before building.
#
# Why: keeps our image "upstream + thin patch layer" instead of a fork.
# Each file in patches/*.patch is a not-yet-merged upstream PR we want early.
#
# Lifecycle:
#   - Add a PR: drop its .patch here (name it <PR-number>-<slug>.patch).
#   - When the PR is merged upstream: delete the .patch, rebuild. No revert needed
#     because the upstream code already contains the change.
#
# This script is safe to run with an empty patches/ dir (no-op).
# Uses `patch` (not git) so it works in minimal build images.
set -eu

PATCH_DIR="${PATCH_DIR:-patches}"

if [ ! -d "$PATCH_DIR" ] || [ -z "$(ls -A "$PATCH_DIR" 2>/dev/null)" ]; then
  echo "[apply-patches] no patches to apply, skipping."
  exit 0
fi

for patch in "$PATCH_DIR"/*.patch; do
  name="$(basename "$patch")"
  echo "[apply-patches] applying $name"
  # --dry-run first: fail loudly if it doesn't apply (e.g. already merged upstream)
  if ! patch -p1 --dry-run --silent < "$patch"; then
    echo "[apply-patches] WARNING: $name does not apply cleanly."
    echo "[apply-patches] It may already be merged upstream — delete it from patches/ if so."
    exit 1
  fi
  patch -p1 < "$patch"
done

echo "[apply-patches] done."
