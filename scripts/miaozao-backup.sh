#!/usr/bin/env bash

set -euo pipefail

DATA_DIR="${DATA_DIR:-data}"
FILES_DIR="${UPLOADS_DIR:-${DATA_DIR}/files}"
BACKUP_ROOT="${BACKUP_ROOT:-${DATA_DIR}/backup}"
MONGODB_URI="${MONGODB_URI:-mongodb://127.0.0.1:27017}"
MONGODB_DB="${MONGODB_DB:-agent_native_runtime}"
VERSION_FILE="${VERSION_FILE:-public/miaozao-version.txt}"

usage() {
  cat <<'EOF'
Usage:
  scripts/miaozao-backup.sh

Environment variables:
  BACKUP_ROOT  Backup root, default data/backup
  DATA_DIR     Data root, default data
  UPLOADS_DIR  File storage directory, default DATA_DIR/files
  MONGODB_URI  MongoDB URI, default mongodb://127.0.0.1:27017
  MONGODB_DB   Database name, default agent_native_runtime
  VERSION_FILE Release version file, default public/miaozao-version.txt
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing command: $1" >&2; exit 1; }
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_cmd mongodump
require_cmd bun

timestamp="$(date +%Y%m%d-%H%M%S)"
backup_dir="${BACKUP_ROOT%/}/${timestamp}"
if [[ -e "$backup_dir" ]]; then
  echo "backup directory already exists: $backup_dir" >&2
  exit 1
fi

mkdir -p "$backup_dir/mongo" "$backup_dir/files"
mongodump --uri "$MONGODB_URI" --db "$MONGODB_DB" --out "$backup_dir/mongo"

if [[ -d "$FILES_DIR" ]]; then
  cp -a "$FILES_DIR/." "$backup_dir/files/"
else
  echo "file directory does not exist, creating an empty backup: $FILES_DIR" >&2
fi

git_commit="$(git rev-parse HEAD 2>/dev/null || true)"
release_version=""
if [[ -f "$VERSION_FILE" ]]; then
  release_version="$(sed -n '1p' "$VERSION_FILE")"
fi

bun -e '
const [output, database, files, createdAt, commit, version] = Bun.argv.slice(1);
const manifest = {
  format_version: 1,
  created_at: createdAt,
  database: { name: database },
  files_directory: files,
  git_commit: commit || null,
  release_version: version || null
};
await Bun.write(output, `${JSON.stringify(manifest, null, 2)}\n`);
' "$backup_dir/manifest.json" "$MONGODB_DB" "$FILES_DIR" "$timestamp" "$git_commit" "$release_version"

echo "Backup created: $backup_dir"
