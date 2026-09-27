#!/usr/bin/env bash
# Dumps the database to ~/llb-backups and keeps the newest 30 dumps.
# Restore: gunzip -c ~/llb-backups/<file>.sql.gz | docker compose exec -T db psql -U llb llb
set -euo pipefail
cd "$(dirname "$0")/.."

dir="$HOME/llb-backups"
mkdir -p "$dir"

if [ -z "$(docker compose ps -q db 2>/dev/null)" ]; then
  echo "backup: db is not running, skipped"
  exit 0
fi

file="$dir/llb-$(date +%F_%H%M%S).sql.gz"
docker compose exec -T db pg_dump -U llb --clean --if-exists llb | gzip > "$file" || { rm -f "$file"; exit 1; }
ls -1t "$dir"/llb-*.sql.gz | tail -n +31 | xargs -r rm --
echo "backup: $file"
