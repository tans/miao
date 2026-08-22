#!/usr/bin/env bash
set -euo pipefail

DATA_DIR="${DATA_DIR:-data}"
FILES_DIR="${UPLOADS_DIR:-${DATA_DIR}/files}"
EXTRACTED_DIR="${EXTRACTED_DIR:-${DATA_DIR}/extracted}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
MONGODB_DB="${MONGODB_DB:-agent_native_runtime}"
DROP_DATABASE=false
BACKUP_DIR=""

usage() {
  cat <<'EOF'
Usage:
  scripts/miaozao-restore.sh BACKUP_DIR [--drop]

Arguments:
  BACKUP_DIR  Backup directory containing mongo.archive, files/, extracted/ and manifest.json
  --drop      Drop existing collections while restoring MongoDB

Environment variables:
  DATA_DIR      Data root, default data
  UPLOADS_DIR  File storage directory, default DATA_DIR/files
  EXTRACTED_DIR Extracted cache directory, default DATA_DIR/extracted
  COMPOSE_FILE  Compose file, default docker-compose.yml
  MONGODB_DB    MongoDB database, default agent_native_runtime
EOF
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

command -v docker >/dev/null 2>&1 || { echo "missing command: docker" >&2; exit 1; }
if [[ ! -d "$BACKUP_DIR" || ! -f "$BACKUP_DIR/manifest.json" || ! -f "$BACKUP_DIR/mongo.archive" || ! -d "$BACKUP_DIR/files" || ! -d "$BACKUP_DIR/extracted" ]]; then
  echo "invalid backup directory: $BACKUP_DIR" >&2
  exit 1
fi
for directory in "$FILES_DIR" "$EXTRACTED_DIR"; do
  case "$directory" in
    ""|"/"|".") echo "refusing unsafe data directory: $directory" >&2; exit 1 ;;
  esac
done

compose=(docker compose -f "$COMPOSE_FILE")
if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
  compose+=(--project-name "$COMPOSE_PROJECT_NAME")
fi
"${compose[@]}" up -d mongo >/dev/null
restore_args=(--archive --db "$MONGODB_DB")
if [[ "$DROP_DATABASE" == true ]]; then
  restore_args+=(--drop)
fi
"${compose[@]}" exec -T mongo mongorestore "${restore_args[@]}" < "$BACKUP_DIR/mongo.archive"

mkdir -p "$FILES_DIR" "$EXTRACTED_DIR"
cp -a "$BACKUP_DIR/files/." "$FILES_DIR/"
cp -a "$BACKUP_DIR/extracted/." "$EXTRACTED_DIR/"

echo "MongoDB restored: $MONGODB_DB"
echo "Files restored to: $FILES_DIR"
echo "Extracted cache restored to: $EXTRACTED_DIR"
if [[ "$DROP_DATABASE" != true ]]; then
  echo "Existing MongoDB collections were preserved; pass --drop to replace them."
fi
