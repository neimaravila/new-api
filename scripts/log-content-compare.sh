#!/usr/bin/env sh
# Prove that log content capture leaves the client's bytes unchanged on the
# tree the image really builds: the committed HEAD without web/, with every
# patches/*.patch applied the way the Dockerfile applies them.
#
# Usage: sh scripts/log-content-compare.sh [--local] [--keep]
#   --local  apply the patches with the local `patch` and test with the local
#            Go, instead of the Dockerfile's golang Alpine image with GNU patch.
#   --keep   keep the temporary tree and print its path.
#
# Only committed files are tested: commit the tests before running this.
set -eu

local_mode=0
keep=0
for arg in "$@"; do
  case "$arg" in
    --local) local_mode=1 ;;
    --keep) keep=1 ;;
    *)
      echo "usage: sh scripts/log-content-compare.sh [--local] [--keep]" >&2
      exit 2
      ;;
  esac
done

root="$(git rev-parse --show-toplevel)"
tree="$(mktemp -d "${TMPDIR:-/tmp}/log-content-compare.XXXXXX")"
cleanup() {
  if [ "$keep" = 1 ]; then
    echo "[log-content-compare] tree kept at $tree"
  else
    rm -rf "$tree"
  fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

echo "[log-content-compare] HEAD $(git -C "$root" rev-parse --short HEAD) -> $tree"
git -C "$root" archive HEAD -- . ':!web' | tar -x -C "$tree"

# Runs inside the patched tree. Every go test runs verbose into a log; the
# named tests must each report PASS, and any other run at least one PASS, so
# a filter that matches nothing fails.
steps='
set -eu
sh scripts/apply-patches.sh
if ! grep -q directForward relay/channel/openai/relay-openai.go; then
  echo "[log-content-compare] patch 7033 is not applied: directForward missing" >&2
  exit 1
fi
echo "[log-content-compare] patch 7033 applied: directForward found"
# counted_test "<tests that must pass>" <go test arguments>
counted_test() {
  required="$1"
  shift
  log="$(mktemp)"
  echo "[log-content-compare] go test -count=1 $*"
  if ! go test -count=1 -v "$@" >"$log" 2>&1; then
    grep -E "^(--- FAIL|FAIL|panic:)|_test\.go:[0-9]+:|Error" "$log" || cat "$log"
    exit 1
  fi
  if grep -q "no tests to run" "$log"; then
    echo "[log-content-compare] no tests to run: $*" >&2
    exit 1
  fi
  for name in $required; do
    if ! grep -q "^--- PASS: $name " "$log"; then
      echo "[log-content-compare] $name did not pass: $*" >&2
      exit 1
    fi
    echo "[log-content-compare] --- PASS: $name"
  done
  passed="$(grep -c "^--- PASS" "$log" || true)"
  if [ "$passed" -eq 0 ]; then
    echo "[log-content-compare] no test ran: $*" >&2
    exit 1
  fi
  grep -E "^ok " "$log"
  echo "[log-content-compare] $passed top-level tests passed"
  rm -f "$log"
}
counted_test "TestLogContentCaptureDoesNotChangeClientBytes TestLogContentCaptureKeepsPingDelivery" \
  ./relay/channel/openai/ -run "LogContent|KeepAliveOverCapture"
counted_test "TestKeepAliveOverCaptureWriter" ./relay/helper/ -run "LogContent|KeepAliveOverCapture"
# Acceptance tests of patches/README.md.
counted_test "" ./relay/channel/openai/
cd relaykit
export GOWORK=off
counted_test "" ./relayconvert/internal/oai_chat/
echo "[log-content-compare] all passed on the patched tree"
'

if [ "$local_mode" = 1 ]; then
  (cd "$tree" && sh -c "$steps")
  exit 0
fi

# The image is read from the Dockerfile so the run follows the build, with GNU
# patch added as the build adds it. The tests run as the host user so the tree
# can be removed afterwards; Go's module and build caches persist on the host.
# -race is left out: the Alpine image has no cgo for it.
image="$(sed -n 's/^FROM \(golang:[^ ]*\) AS builder2$/\1/p' "$root/Dockerfile")"
if [ -z "$image" ]; then
  echo "[log-content-compare] golang build image not found in Dockerfile" >&2
  exit 1
fi
runner="new-api-log-content-compare:patch"
printf 'FROM %s\nRUN for i in 1 2 3; do apk add --no-cache patch && break || sleep 3; done && command -v patch\n' "$image" |
  docker build -q -t "$runner" - >/dev/null
cache="${XDG_CACHE_HOME:-$HOME/.cache}/new-api-log-content-compare"
mkdir -p "$cache/gomod" "$cache/gobuild"
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -v "$tree:/build" -w /build \
  -v "$cache/gomod:/gomod" -v "$cache/gobuild:/gobuild" \
  -e HOME=/tmp -e GOMODCACHE=/gomod -e GOCACHE=/gobuild \
  -e CGO_ENABLED=0 -e GOWORK=off -e GOEXPERIMENT=greenteagc \
  "$runner" \
  sh -c "$steps"
