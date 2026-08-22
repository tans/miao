#!/usr/bin/env bash

set -euo pipefail

DATA_DIR="${DATA_DIR:-data}"
FILES_DIR="${UPLOADS_DIR:-${DATA_DIR}/files}"
MONGODB_URI="${MONGODB_URI:-mongodb://127.0.0.1:27017}"
MONGODB_DB="${MONGODB_DB:-agent_native_runtime}"
DROP_DATABASE=false
BACKUP_DIR=""

usage() {
  cat <<'EOF'
Usage:
  scripts/miaozao-restore.sh BACKUP_DIR [--drop]

Arguments:
  BACKUP_DIR  Backup directory containing mongo/, files/ and manifest.json
  --drop      Drop existing collections while restoring MongoDB

Environment variables:
  DATA_DIR     Data root, default data
  UPLOADS_DIR  File storage directory, default DATA_DIR/files
  MONGODB_URI  MongoDB URI, default mongodb://127.0.0.1:27017
  MONGODB_DB   Database name, default agent_native_runtime
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing command: $1" >&2; exit 1; }
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" || $# -lt 1 ]]; then
  usage
  [[ $# -ge 1 ]] && exit 0 || exit 1
fi

BACKUP_DIR="$1"
shift
while [[ $# -gt 0 ]]; do
  case "$1" in
    --drop) DROP_DATABASE=true ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 1 ;;
  esac
  shift
done

require_cmd mongorestore

if [[ ! -d "$BACKUP_DIR" || ! -f "$BACKUP_DIR/manifest.json" || ! -d "$BACKUP_DIR/mongo" || ! -d "$BACKUP_DIR/files" ]]; then
  echo "invalid backup directory: $BACKUP_DIR" >&2
  exit 1
fi
case "$FILES_DIR" in
  ""|"/"|".") echo "refusing unsafe file directory: $FILES_DIR" >&2; exit 1 ;;
esac

mongo_args=(--uri "$MONGODB_URI" --db "$MONGODB_DB")
if [[ "$DROP_DATABASE" == true ]]; then
  mongo_args+=(--drop)
fi
mongorestore "${mongo_args[@]}" "$BACKUP_DIR/mongo/$MONGODB_DB"

mkdir -p "$FILES_DIR"
cp -a "$BACKUP_DIR/files/." "$FILES_DIR/"

echo "MongoDB restored: $MONGODB_DB"
echo "Files restored to: $FILES_DIR"
if [[ "$DROP_DATABASE" != true ]]; then
  echo "Existing MongoDB collections were preserved; pass --drop to replace them."
fi
