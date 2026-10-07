#!/usr/bin/env bash
set -Eeuo pipefail

# Server-side release switcher. The release directory must already contain
# the committed source archive and a Linux amd64 `miao` binary.
BASE_DIR="${MIAO_BASE_DIR:-/data/miao-platform}"
OPS_DIR="$BASE_DIR/ops"
RUN="$OPS_DIR/run"
CURRENT="$BASE_DIR/current"
RELEASES="$BASE_DIR/releases"
DEPLOYED="$BASE_DIR/DEPLOYED_COMMIT"
DOMAIN="${MIAO_PUBLIC_HOST:-miao.minapp.xin}"
SHA="${1:-}"

die() { echo "deploy: $*" >&2; exit 1; }
[[ "$SHA" =~ ^[0-9a-f]{7,64}$ ]] || die "usage: deploy.sh <commit-sha>"
[[ "$(id -u)" == 0 ]] || die "run as root"
command -v flock >/dev/null || die "flock is required"
[[ -x "$RUN" ]] || die "missing helper: $RUN"

exec 9>"$OPS_DIR/.deploy.lock"
flock -n 9 || die "another deployment is running"

release="$RELEASES/$SHA"
[[ -d "$release" && -x "$release/miao" ]] || die "release is incomplete: $release"
[[ -x "$release/scripts/install.sh" ]] || die "release is missing scripts/install.sh"
version="$($release/miao version 2>/dev/null || true)"
[[ "$version" == *"$SHA"* ]] || die "binary version does not match $SHA"

previous_target="$(readlink -f "$CURRENT")"
previous_release="$(basename "$previous_target")"
previous_sha="$(cat "$DEPLOYED" 2>/dev/null || true)"
[[ -d "$previous_target" ]] || die "current release is missing: $previous_target"

if [[ "$previous_release" == "$SHA" ]]; then
  echo "deploy: $SHA is already current"
  exit 0
fi

rollback() {
  set +e
  echo "deploy: rolling back to $previous_release" >&2
  "$RUN" npm run server:stop >/dev/null 2>&1
  rollback_link="$BASE_DIR/current.rollback.$$"
  ln -s "releases/$previous_release" "$rollback_link"
  chown -h miao-platform:miao-platform "$rollback_link"
  mv -Tf "$rollback_link" "$CURRENT"
  "$RUN" npm run server:start >/dev/null 2>&1
  if [[ -n "$previous_sha" ]]; then
    printf '%s\n' "$previous_sha" > "$DEPLOYED"
    chown miao-platform:miao-platform "$DEPLOYED"
  fi
  echo "deploy: rollback completed; database restore is manual" >&2
}

echo "deploy: backing up current release"
"$RUN" npm run backup

echo "deploy: installing $SHA"
chown -R miao-platform:miao-platform "$release"
"$RUN" bash "$release/scripts/install.sh" --binary "$release/miao"

next_link="$BASE_DIR/current.next.$$"
ln -s "releases/$SHA" "$next_link"
chown -h miao-platform:miao-platform "$next_link"
mv -Tf "$next_link" "$CURRENT"

if ! "$RUN" npm run server:start; then
  rollback
  exit 1
fi

port="$(awk -F= '$1 == "MIAO_PORT" { value=$2 } END { print value }' "$BASE_DIR/install/miao.env")"
port="${port:-41879}"
healthy=0
for _ in $(seq 1 30); do
  if curl --noproxy '*' --fail --silent "http://127.0.0.1:$port/api/health" | grep -q '"service":"miao"'; then
    healthy=1
    break
  fi
  sleep 1
done
if [[ "$healthy" != 1 ]] || ! curl --noproxy '*' --fail --silent --show-error "https://$DOMAIN/api/health" | grep -q '"service":"miao"'; then
  rollback
  exit 1
fi

printf '%s\n' "$SHA" > "$DEPLOYED"
chown miao-platform:miao-platform "$DEPLOYED"
echo "deploy: $SHA is live at https://$DOMAIN"
