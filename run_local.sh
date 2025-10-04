#!/usr/bin/env bash
set -euo pipefail

if [ ! -f ".env.wizard" ]; then
  echo "File .env.wizard tidak ditemukan. Salin dari .env.wizard.example terlebih dahulu."
  exit 1
fi

if [ ! -f ".env.userbot" ]; then
  echo "File .env.userbot tidak ditemukan. Salin dari .env.userbot.example terlebih dahulu."
  exit 1
fi

echo "Menjalankan stack lokal (wizard + orchestrator + postgres)..."
docker compose -f docker-compose.local.yml up --build
