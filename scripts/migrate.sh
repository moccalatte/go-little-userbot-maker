#!/usr/bin/env bash
set -euo pipefail

DB_URL=${DB_URL:-postgres://userbot:userbot@localhost:5432/little_userbot?sslmode=disable}
MIGRATIONS_DIR=${MIGRATIONS_DIR:-userbot/migrations}

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker diperlukan untuk menjalankan script migrasi sederhana ini."
  exit 1
fi

docker run --rm -v "$(pwd)/${MIGRATIONS_DIR}:/migrations" --network host migrate/migrate:v4.17.1 \
  -path=/migrations -database "${DB_URL}" up
