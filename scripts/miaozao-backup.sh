#!/usr/bin/env bash
set -euo pipefail

DATA_DIR="${DATA_DIR:-data}"
FILES_DIR="${UPLOADS_DIR:-${DATA_DIR}/files}"
EXTRACTED_DIR="${EXTRACTED_DIR:-${DATA_DIR}/extracted}"
BACKUP_ROOT="${BACKUP_ROOT:-${DATA_DIR}/backup}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
MONGODB_DB="${MONGODB_DB:-agent_native_runtime}"
VERSION_FILE="${VERSION_FILE:-public/miaozao-version.txt}"

usage() {
  cat <<'EOF'
Usage:
  scripts/miaozao-backup.sh

Environment variables:
  BACKUP_ROOT   Backup root, default data/backup
  DATA_DIR      Data root, default data
  UPLOADS_DIR   File storage directory, default DATA_DIR/files
  EXTRACTED_DIR Extracted cache directory, default DATA_DIR/extracted
  COMPOSE_FILE  Compose file, default docker-compose.yml
  MONGODB_DB    MongoDB database, default agent_native_runtime
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi
command -v docker >/dev/null 2>&1 || { echo "missing command: docker" >&2; exit 1; }

compose=(docker compose -f "$COMPOSE_FILE")
if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
  compose+=(--project-name "$COMPOSE_PROJECT_NAME")
fi

timestamp="$(date +%Y%m%d-%H%M%S)"
backup_dir="${BACKUP_ROOT%/}/${timestamp}"
if [[ -e "$backup_dir" ]]; then
  echo "backup directory already exists: $backup_dir" >&2
  exit 1
fi

mkdir -p "$backup_dir/files" "$backup_dir/extracted"
"${compose[@]}" up -d mongo runtime >/dev/null
"${compose[@]}" exec -T mongo mongodump --db "$MONGODB_DB" --archive > "$backup_dir/mongo.archive"

if [[ -d "$FILES_DIR" ]]; then
  cp -a "$FILES_DIR/." "$backup_dir/files/"
else
  echo "file directory does not exist, creating an empty backup: $FILES_DIR" >&2
fi
if [[ -d "$EXTRACTED_DIR" ]]; then
  cp -a "$EXTRACTED_DIR/." "$backup_dir/extracted/"
else
  echo "extracted directory does not exist, creating an empty cache backup: $EXTRACTED_DIR" >&2
fi

release_version=""
if [[ -f "$VERSION_FILE" ]]; then
  release_version="$(sed -n '1p' "$VERSION_FILE")"
fi
manifest_json="$("${compose[@]}" exec -T runtime bun -e '
const [database, files, extracted, createdAt, version] = Bun.argv.slice(1);
console.log(JSON.stringify({
  format_version: 2,
  created_at: createdAt,
  database: { name: database },
  files_directory: files,
  extracted_directory: extracted,
  git_commit: process.env.GIT_COMMIT || null,
  release_version: version || null
}, null, 2));
' "$MONGODB_DB" "$FILES_DIR" "$EXTRACTED_DIR" "$timestamp" "$release_version")"
printf '%s\n' "$manifest_json" > "$backup_dir/manifest.json"

echo "Backup created: $backup_dir"
