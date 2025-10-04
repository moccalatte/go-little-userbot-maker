#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p storage/logs/wizard storage/logs/userbot

export APP_NAME=${APP_NAME:-go-little-userbot-maker}
export APP_ENV=${APP_ENV:-development}
export LOG_LEVEL=${LOG_LEVEL:-debug}
export LOG_JSON=${LOG_JSON:-true}

export WIZARD_USE_MOCK=${WIZARD_USE_MOCK:-true}
export WIZARD_BOT_TOKEN=${WIZARD_BOT_TOKEN:-dummy-token}
export WIZARD_ORCHESTRATOR_URL=${WIZARD_ORCHESTRATOR_URL:-http://localhost:8080}
export WIZARD_STATE_TTL=${WIZARD_STATE_TTL:-5m}
export WIZARD_STORAGE_PATH=${WIZARD_STORAGE_PATH:-./storage/logs/wizard}

export ORCH_LISTEN_ADDR=${ORCH_LISTEN_ADDR:-:8080}
export ORCH_SECRET_KEY=${ORCH_SECRET_KEY:-development-secret-key}
export ORCH_HEALTH_INTERVAL=${ORCH_HEALTH_INTERVAL:-30s}
export ORCH_ENABLE_MOCK=${ORCH_ENABLE_MOCK:-true}
export ORCH_MAX_WORKERS=${ORCH_MAX_WORKERS:-16}

echo "[dev-mock] Menjalankan orchestrator (mock mode)..."
go run ./cmd/userbot &
orch_pid=$!
wizard_pid=0

cleanup() {
  echo "[dev-mock] Menghentikan layanan..."
  if [ "$orch_pid" -ne 0 ]; then
    kill "$orch_pid" 2>/dev/null || true
  fi
  if [ "$wizard_pid" -ne 0 ]; then
    kill "$wizard_pid" 2>/dev/null || true
  fi
}

trap cleanup EXIT INT TERM

sleep 2

echo "[dev-mock] Menjalankan bot wizard (mock bot)..."
go run ./cmd/botwizard &
wizard_pid=$!

wait "$orch_pid" "$wizard_pid"
