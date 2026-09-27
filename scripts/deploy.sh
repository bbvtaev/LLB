#!/usr/bin/env bash
# Deploys the bot to a server: ./scripts/deploy.sh root@1.2.3.4
# Copies the code to ~/llb, backs up the database, then rebuilds and restarts the bot.
# The database lives in the llb_pgdata Docker volume and is never touched by a deploy.
set -euo pipefail
cd "$(dirname "$0")/.."

host="${1:-${LLB_HOST:-}}"
[ -n "$host" ] || { echo "usage: $0 user@host (or set LLB_HOST)"; exit 1; }
dir="llb"

rsync -az --delete --exclude .env --exclude .git --exclude /llb ./ "$host:$dir/"

# .env is copied only once, so edits made on the server are not overwritten.
if ! ssh "$host" "test -f $dir/.env"; then
  scp .env "$host:$dir/.env"
  echo "copied .env"
fi

ssh "$host" "cd $dir && ./scripts/backup.sh && docker compose up -d --build && docker compose ps"
