# Go Little Userbot Maker

Ekosistem dua layanan Telegram berbasis Go 1.22 untuk mereplikasi pengalaman Little Userbot Maker: **Bot Wizard** sebagai antarmuka percakapan dan **Userbot Orchestrator** sebagai backend multi-session `gotd/td` dengan Postgres.

## Arsitektur Singkat
- **Bot Wizard (`cmd/botwizard`)** memakai `go-telegram-bot-api/v5`, menyajikan menu ReplyKeyboard, menyimpan state dengan TTL, serta menulis transcript JSONL di `storage/logs/wizard/<telegram_id>/`.
- **Userbot Orchestrator (`cmd/userbot`)** mengeksekusi worker per sesi, mengenkripsi session string dengan AES-GCM, menyimpan metadata di PostgreSQL, dan mengekspos HTTP API (`/sessions`, `/features/...`, `/stats/database`, `/healthz`).
- Observabilitas: log terstruktur `zap`, metrik Prometheus di port 9090, skrip migrasi SQL di `userbot/migrations`.

## Persyaratan
- Go 1.22+
- Docker & Docker Compose (opsional untuk stack penuh)
- Akses token bot Telegram + kredensial Postgres

## Langkah Cepat
1. **Siapkan konfigurasi**
   ```bash
   cp .env.wizard.example .env.wizard
   cp .env.userbot.example .env.userbot
   # edit nilai BOT_TOKEN, ORCH_SECRET_KEY, dsb.
   ```
   Contoh `.env` bawaan sudah mengaktifkan mode mock; set `WIZARD_USE_MOCK=false` dan `ORCH_ENABLE_MOCK=false` bila siap tersambung ke Telegram & Postgres nyata.
2. **Mode belajar (tanpa Postgres/Telegram)**
   ```bash
   make dev-mock
   ```
   Perintah ini menjalankan orchestrator in-memory dan wizard mock, cocok untuk eksplorasi awal.
3. **Jalankan tes & build lokal**
   ```bash
   go test ./...
   make build
   ```
4. **Stack lokal lengkap (wizard + orchestrator + Postgres + Redis)**
   ```bash
   ./run_local.sh            # blocking mode
   # atau
   make compose-up-local     # mode daemon
   ```
5. **Migrasi database** (jalankan setelah stack siap)
   ```bash
   DB_URL=postgres://userbot:userbot@localhost:5432/little_userbot?sslmode=disable \
   ./scripts/migrate.sh
   ```

## Struktur Penting
```
cmd/                Entrypoint wizard & orchestrator
internal/config     Loader environment & struktur konfigurasi
internal/wizard     State machine bot, transcript writer, klien orchestrator
internal/orchestrator Session manager, registry command, HTTP API, worker
userbot/migrations  Skema PostgreSQL (users, sessions, reply_guard, dsb.)
docker-compose*.yml Stack lokal & produksi terpisah
scripts/            Skrip bantu (`dev_mock.sh`, `migrate.sh`)
```

## Operasional
- **Wizard** berjalan di port `WIZARD_LISTEN_ADDR` (default 8081) dan logging JSON ke stdout serta file transcript.
- **Orchestrator** membuka port `ORCH_LISTEN_ADDR` (default 8080) & Prometheus di `ORCH_METRICS_ADDR`.
- Gunakan `make run-wizard` / `make run-userbot` untuk mode pengembangan tanpa Docker.
- Gunakan `make dev-mock` bila ingin memulai tanpa dependensi eksternal.
- Volume log: `wizard_logs`, `userbot_logs`; backup `userbot_db_data` untuk Postgres.

## Verifikasi
- `go test ./...`
- Endpoint health: `curl http://localhost:8080/healthz`
- Log wizard: periksa `storage/logs/wizard/<telegram_id>/YYYY-MM-DD.jsonl`.

## Rollback Singkat
- Hentikan layanan (`docker compose ... down`).
- Restore backup database (volume `userbot_db_data`).
- Deploy kembali image sebelumnya (tag versi lama) lalu jalankan migrasi `down` bila diperlukan dengan `scripts/migrate.sh`.
