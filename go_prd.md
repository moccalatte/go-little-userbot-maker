# Little Userbot Maker – Go/GoTD PRD

## 1. Tujuan Produk
- **Objective**: Mereplikasi pengalaman Little Userbot Maker sebagai ekosistem dua-bot Telegram (wizard sebagai “frontend” percakapan dan userbot otomatis sebagai “backend”) memakai stack Go 1.22 (`go-telegram-bot-api` + `gotd/td`).
- **Why Go**: concurrency ringan, deployment single binary, integrasi lebih mudah dengan infrastruktur container.

## 2. High-Level Architecture (Non-Web)
```
┌────────────────────────┐          ┌────────────────────────┐
│        VPS A           │          │        VPS B           │
│   docker-compose       │          │   docker-compose       │
│   stack: bot wizard    │          │   stack: userbot + DB  │
└──────────┬─────────────┘          └──────────┬─────────────┘
           │ HTTP/gRPC/queue                     │
           ▼                                     ▼
   [Bot Wizard Service]                 [Userbot Orchestrator]
      (Go + go-telegram-bot-api)          (Go + gotd/td)
                                                   │
                                                   ▼
                                          [PostgreSQL 15]
```
- **Bot Wizard** bertindak sebagai conversational UI + admin panel sepenuhnya di Telegram (tidak ada antarmuka web) dan berjalan terisolasi di VPS A.
- **Userbot Orchestrator** menjalankan banyak session userbot menggunakan `gotd/td`, berada di VPS B bersama PostgreSQL.
- **Shared Services**: logging terstruktur (zap/logrus), tracing (OpenTelemetry opsional). Log forwarding bisa dikirim ke stack monitoring terpisah melalui exporter ringan.

### 2.1 Deployment Model & Directory Layout
- **Folder Top-Level** (`/home/app/go-little-userbot-maker` contoh):
  - `bot-wizard/` → kode bot wizard Go + Dockerfile khusus stack ini.
  - `userbot/` → kode orchestrator + migrasi PostgreSQL + Dockerfile.
  - `scripts/` → utilitas (`dev_mock.sh`, `migrate.sh`) untuk mode belajar dan migrasi.
  - `docker-compose.local.yml` → menjalankan kedua layanan + Postgres untuk testing di mesin lokal.
  - `docker-compose.wizard.yml` → stack produksi untuk VPS A (bot wizard only) dengan service reverse proxy opsional.
  - `docker-compose.userbot.yml` → stack produksi untuk VPS B (orchestrator + PostgreSQL + worker tambahan bila perlu).
  - `Makefile` atau skrip shell kecil (`./run_local.sh`) untuk mempermudah developer pemula/AI mengeksekusi operasi umum (build, test, lint, compose).
- **Volume Data**: direktori data PostgreSQL tidak disimpan di repo; gunakan volume bernama (`userbot_db_data`) agar mudah dipindah antar VPS dan bisa di-backup.
- **Environment Files**: simpan `.env.wizard` dan `.env.userbot` terpisah; setiap compose file memanggil env masing-masing. Sertakan contoh di repo (`.env.example`).
- **Networking**: VPS A dan B berkomunikasi lewat jaringan privat/zero-tier/WireGuard. Setiap compose expose port internal (contoh 8080) dan diamankan dengan firewall + mTLS/token.

### 2.2 Workflow Pengembangan
- **Beginner Mock Mode**: jalankan `make dev-mock` (alias `scripts/dev_mock.sh`) untuk menyalakan orchestrator in-memory + wizard mock. Tidak perlu Postgres atau bot token; seluruh log tetap tercatat di stdout dan `storage/logs`.
- **Local Testing Penuh**: jalankan `docker compose -f docker-compose.local.yml up --build` untuk menjalankan wizard, orchestrator, dan Postgres di mesin lokal (menggunakan bridge network yang sama). Wizard bicara ke orchestrator via hostname compose (`http://userbot:8080`).
- **Staging/Production**:
  - Deploy ke VPS A: `docker compose -f docker-compose.wizard.yml up -d`. Hanya service wizard + optional log forwarder.
  - Deploy ke VPS B: `docker compose -f docker-compose.userbot.yml up -d`. Termasuk orchestrator, Postgres, migrator, worker scheduler.
- **CI/CD-friendly**: setiap image dibangun terpisah (`bot-wizard:latest`, `userbot-orchestrator:latest`). Tag dengan hash git untuk rollback cepat.

### 2.3 Logging & Audit Topology
- **Tujuan**: seluruh interaksi (terminal output, percakapan wizard, aksi admin/owner, aktivitas userbot) terekam dengan struktur konsisten agar mudah dibaca developer/AI ketika debugging.
- **Lapisan Logging**:
  - *Service stdout/stderr*: struktur JSON line log (`{"ts":"...","level":"info","service":"wizard","event":"..."}`) yang otomatis dikumpulkan oleh Docker logging driver dan diteruskan real-time ke agen (`promtail`/`filebeat`) → log sink (contoh: Loki/ELK). Tidak perlu tail manual di server untuk pemantauan rutin.
  - *Conversation transcripts*: setiap percakapan wizard disalin ke `storage/logs/wizard/<telegram_id>/<YYYY-MM-DD>.jsonl` (rotasi harian). Simpan payload request/response, tombol yang dipilih, serta context state.
  - *Userbot activity*: aksi per-userbot (command, schedule, broadcast) dicatat ke `storage/logs/userbot/<telegram_id>.jsonl`. Sertakan field `origin` (user/manual/automation).
  - *Admin/Owner actions*: tindakan seperti `DELETE /sessions`, perubahan konfigurasi, health check manual, disimpan di `storage/logs/admin/audit.jsonl` dan di-post ke channel Telegram admin.
- **Retention**: minimal 30 hari di VPS (rolling); untuk produksi gunakan sink terpusat dengan retensi 90+ hari. Pastikan folder log masuk volume Docker (`wizard_logs`, `userbot_logs`).
- **Self-Serve Access**: sediakan command admin `/logs <telegram_id>` yang men-trigger signed URL atau cuplikan 20 baris terakhir agar AI/operator bisa ambil log cepat tanpa SSH.
- **Redaction**: masker data sensitif (OTP, 2FA, session string) sebelum menulis ke log; simpan hash SHA-256 untuk korelasi tanpa menampilkan nilai mentah.

## 3. Bot Wizard Requirements (Go)
-### 3.1 Conversational Flow
- Library: `github.com/go-telegram-bot-api/telegram-bot-api/v5` atau `github.com/PaulSonOfLars/gotgbot` (pilih salah satu yang mendukung ReplyKeyboard).
- Semua menu dari `/start` sampai admin menu harus memakai ReplyKeyboardMarkup/InlineKeyboardMarkup.
- State machine disimpan dalam in-memory map dengan TTL, keyed by chat_id. Mode mock default memakai penyimpanan in-memory.
- Logging aktivitas:
  - Stdout/stderr dalam format JSON terstruktur (level, event, chat_id, trace_id) agar mudah dikirim ke log sink.
  - Simpan transcript interaksi ke `storage/logs/wizard/<telegram_id>/<YYYY-MM-DD>.jsonl` termasuk pesan user, respon wizard, keyboard yang ditampilkan, dan state aktif.
  - Tandai event penting (`event_type`: onboarding, login, admin_action) supaya query cepat.
  - Lampirkan `session_hash` dan `request_id` untuk menghubungkan log wizard ↔ orchestrator.

#### Menu Utama `/start`
- Tombol: `🤖 Buat Userbot`, `🔑 Token Login`, `⚙️ Kelola Userbot`, `🔧 Admin Settings` (conditional).
- Handler memastikan `ResetUserState(chatID)` dan catat log start.

#### Buat Userbot Flow
1. Pilih metode `📱 OTP` / `📷 QR` / `🔙 Back`.
2. OTP: nomor telepon → API ID → API hash → OTP (format validasi) → 2FA password.
3. QR: API ID/API hash → generate QR (pakai `github.com/skip2/go-qrcode`) → monitoring goroutine menunggu auth → fallback manual.
4. Setelah session siap, kirim string (opsional masked) ke orchestrator melalui kanal internal (contoh: `POST /sessions`).
5. Payload minimal: `telegram_id`, `session_string`, `login_method`, `metadata` (DC, platform). Wizard menyimpan log audit untuk trace.
 6. Untuk mode mock, wizard membangkitkan session string dummy terenkripsi agar alur dapat diuji tanpa Telegram.

#### Token Login
- Validasi base64 panjang > 100, decode test.
- Hapus sesi lama via orchestrator sebelum insert baru.

#### Kelola Userbot
- Requires `Get /users/{id}` untuk memastikan status userbot aktif.
- ReplyKeyboard:
  - `📋 Lihat Commands` → show per-command keyboard.
  - `🤖 Reply Guard`, `📢 Broadcast Scheduler`, `💬 Get Group Info`, `ℹ️ System Info`, `❓ Help Command`.
  - Flow configuration menulis ke orchestrator (`PATCH /features/{type}`) dengan payload JSON.

#### Admin Settings (Telegram Admin Frontend)
- Tombol: `🗑️ Clean Database`, `📊 Database Stats`, `👥 List Sessions`, `🔍 Debug Report`, `🚑 Health Check`, `📈 Performance Logs`, `🧪 Automated Testing`, `🔙 Back`.
- Endpoint backend (internal, non-publik):
  - `DELETE /sessions/{user_id}` (konfirmasi dulu via keyboard).
  - `GET /stats/database`, `GET /stats/logs`, `POST /automation/run-tests`.
- Semua aksi log di level INFO dan ERROR.

### 3.2 Error Handling
- Gunakan context dengan timeout (misal 15 detik) untuk setiap panggilan orchestrator/Telegram API.
- Retries exponential backoff untuk Telegram error `Too Many Requests`.
- Panic harus dilindungi `recover` dan dicatat ke Sentry/Otel optional.

## 4. Userbot Orchestrator (GoTD)
### 4.1 Session Management
- Library: `github.com/gotd/td`.
- Setiap session dijalankan dalam goroutine terisolasi dengan worker pool.
- Session string disimpan terenkripsi (AES-GCM) di PostgreSQL. Ketika `ORCH_ENABLE_MOCK=true`, data disimpan di store in-memory dengan log heartbeat tetap aktif.
- Hook internal untuk menerima session baru dari wizard:
  - Endpoint `POST /sessions`: decrypt, simpan, spawn userbot worker.
  - Endpoint `DELETE /sessions/{user}`: hentikan worker, cleanup data.
- Worker diidentifikasi dengan `telegram_id` → satu userbot = satu worker. Saat service start, lakukan bootstrap: load semua sesi aktif dari tabel `sessions`, decrypt, dan spawn worker agar uptime konsisten.
- Scheduler internal melakukan health-check tiap X detik (configurable) untuk memastikan worker masih konek; jika disconnect, coba reconnect dengan backoff.

### 4.2 Command Framework
- Standardize interface `Command` dengan metode `Name()`, `Execute(ctx, payload)`, `Help()`. Registrasikan via `registry` map.
- Commands wajib: `help`, `info`, `gg` (Google), `sg` (broadcast scheduler), `rg` (reply guard), `status`.
- Reply guard rule disimpan di tabel `reply_guard_rules` dengan kolom JSONB include/exclude/targets.
- Broadcast scheduler memanfaatkan cron worker (misal `github.com/robfig/cron/v3`).

### 4.3 Monitoring & Telemetry
- Logs terstruktur (zap) dengan key `user_id`, `telegram_id`, `command`, `actor` (user/automation/admin), `latency_ms`, `request_id`. Setiap eksekusi command tulis outcome (success/fail) + alasan.
- Scheduler, worker pool, dan event penting (login, logout, restart) log ke `storage/logs/userbot/<telegram_id>.jsonl` untuk korelasi dengan wizard.
- Health endpoint `/healthz` memeriksa koneksi database, queue, dan goroutine leak (optional stack dump).

## 5. Data Model (PostgreSQL)
- **users**: id (bigint), telegram_id (unique), username, full_name, plan, status, timestamps.
- **sessions**: id, user_id, owner_user_id, telegram_id, session_type, bot_username, bot_display_name, session_encrypted, secret_version, login_method, metadata JSONB (device, DC, ip), features JSONB, config JSONB, worker_host, status, session_hash, request_id, origin, created_at, updated_at, last_seen.
- **bot_commands**: id, session_id, command, description, response_type, payload JSONB, enabled, timestamps.
- **bot_command_triggers**: id, command_id, trigger_type (command/text/regex/button), trigger_value, metadata JSONB, timestamps.
- **wizard_runs**: id, user_id, session_id, status (draft/in_progress/completed/failed/aborted), current_step, payload JSONB, timestamps, completed_at.
- **wizard_steps**: id, wizard_run_id, step_name, sequence, state JSONB, timestamps.
- **audits**: id, actor_id, target_type, target_id, action, diff JSONB, metadata JSONB, created_at.
- **reply_guard_rules**: id, user_id, include_keywords, exclude_keywords, regex, targets, reply_text, status.
- **broadcast_jobs**: id, user_id, message, interval_minutes, targets_json, next_run, enabled.
- **usage_stats**: id, user_id, session_id, command, count, last_used.
- Gunakan migrations dengan `golang-migrate` dan idempotent seed default command (melalui wizard persistence) setelah sesi dibuat.

### 5.1 Observability & Reliability Practices (Go-centric)
- **Structured Logging**: gunakan `zap`/`zerolog` dengan field wajib (`user_id`, `chat_id`, `command`, `flow_state`, `trace_id`). Bungkus handler Telegram dan orchestrator dengan middleware logging.
- **Context Propagation**: semua request internal membawa `context.Context` dengan deadline & nilai `trace_id`; pastikan worker goroutine menghormati cancel.
- **Error Taxonomy**: bedakan MTProto errors (FloodWait, AuthKeyUnregistered, SessionPasswordNeeded) dan network I/O. Implementasikan retry/backoff sesuai `gotd/td` best practice (`telegram.Options{RetryInterval, MaxRetries}`).
- **Session Lifecycle Hooks**: manfaatkan `gotd/td/telegram` `Reconnect` dan `Updates` handler untuk memastikan DC migration, bad salt, dan auth key refresh otomatis; log peristiwa tersebut di level WARN.
- **Panic & Leak Detection**: bungkus goroutine dengan `defer` recovery, jalankan `go test -race`, dan tampilkan health endpoint yang memeriksa jumlah goroutine abnormal.
- **Audit Trail**: simpan catatan JSON per perubahan konfigurasi (Reply Guard, Broadcast) dengan checksum agar memudahkan rollback.
- **Continuous Verification**: buat synthetic check (misal job kecil yang memicu command dummy) jalankan via Cron dan laporkan hasil ke admin bot. Mode mock dapat dipakai sebagai pre-check sebelum produksi.
- **Log Storage & Access**: arahkan semua stdout/stderr ke log collector (Loki/ELK); mount volume `storage/logs` untuk transcript & audit; sediakan skrip `./scripts/tail_logs.sh wizard <telegram_id>` hanya sebagai opsi darurat bila akses log sink bermasalah.

## 6. Future Enhancements
1. **Telegram Admin Toolkit**: paket command/menu lanjutan yang menampilkan metrics, logs, dan kontrol admin langsung di Telegram (opsional CLI).
2. **Template Marketplace**: user bisa impor/ekspor preset Reply Guard & Broadcast.
3. **Flow Automation**: drag-and-drop workflow builder (Zapier-like) berbasis event userbot.
4. **Alerting**: Integrasi Telegram admin channel untuk error critical + grafana alert.
5. **Multi-language**: dukung i18n untuk teks wizard dan command.
6. **Reseller API**: gRPC endpoints untuk provisioning mass user.
7. **Zero-Downtime Rollout**: Blue/green deploy orchestrator dengan session drain.
8. **Config Versioning Service**: simpan konfigurasi feature di storage terpusat dengan versioning (GitOps style) untuk memastikan perubahan mudah ditinjau & rollback.

## 7. Delivery Checklist
- [ ] Environment vars (BOT_TOKEN, DATABASE_URL, SECRET_KEY, API keys) diatur di kedua layanan; untuk mode mock cukup memastikan nilai default aman.
- [ ] Mode mock (`make dev-mock`) berjalan lancar dan menulis log ke stdout serta `storage/logs`.
- [ ] Unit test untuk flow OTP, QR, token login, admin commands, reply guard sync.
- [ ] Integration test memverifikasi orchestrator menerima session baru dan menjalankan command minimal.
- [ ] CI pipeline: lint (golangci-lint), test, static analysis, docker build.

## 8. Risks & Mitigations
- **Telegram rate limits**: implementasi backoff dan queue.
- **Session leakage**: wajib encryption + secret rotation plan.
- **Process crash**: jalankan orchestrator di supervisor (systemd/k8s) dengan readiness probe.
- **Network restrictions**: sediakan mode proxy (MTProto) jika host memblokir Telegram.

## 9. Implementation Guidelines (Untuk AI/Developer)
- **Bahasa & Versi**: Go 1.22.x. Modul dibagi minimal menjadi `cmd/botwizard`, `cmd/userbot`, `internal/<paket>` untuk handler, storage, logging.
- **Dependensi**:
  - Bot wizard: `go-telegram-bot-api` v5 atau `gotgbot` terbaru.
  - Userbot: `github.com/gotd/td` (+ `telegram` client, `mtproto` helpers), `github.com/gotd/neo` untuk helper.
  - Observability: `go.uber.org/zap`, optional `github.com/getsentry/sentry-go`.
- **Project Layout**:
  - `internal/wizard/` (handlers, state, keyboards, logging middleware).
  - `internal/orchestrator/` (session manager, command registry, workers).
  - `pkg/storage/` (PostgreSQL repo menggunakan `pgx`), `internal/config/` (env loader), `pkg/logging/` (zap setup).
- **State & Context**: gunakan `context.Context` pada setiap handler; simpan state wizard dalam map + mutex untuk prototipe.
- **Error Handling**: bungkus operasi Telegram/DB; gunakan error wrapping (`fmt.Errorf("...: %w", err)`) agar mudah di-trace.
- **Logging**: setiap handler log `event`, `user_id`, `chat_id`, `trace_id`, `request_id`. Wizard harus menulis transcript percakapan per user, sedangkan orchestrator menyimpan log per `telegram_id` dan mirror ke stdout untuk collector. Sediakan helper logger agar format konsisten lintas service.
- **MTProto Practices**: gunakan `telegram.Options{DCList, Signal, RetryIf}` untuk reliabilitas; perhatikan perbedaan fatal error vs retryable (bad salt, flood wait, deactivated user).
- **Testing**: jalankan `go test ./...` + race detector. Buat integration test yang memanggil wizard handler secara lokal. Sediakan `docker-compose` untuk Postgres.
- **Config Management**: gunakan `envconfig` atau `viper` untuk memuat env; simpan secrets di `SECRET_KEY`, `BOT_TOKEN`, etc. Hindari menaruh default sensitif.
- **Build & Deploy**: gunakan `go build` menghasilkan binary terpisah (wizard/orchestrator). Sertakan Dockerfile multi-stage.
- **Docker Compose Guardrails**: dokumentasikan command di README/PRD (contoh: `make up-wizard`, `make up-userbot`) agar pemula bisa cepat repeatable dan AI mudah menjalankan ulang.

## 10. Development Workflow & QA
1. **Linting**: `golangci-lint run` (include govet, revive, staticcheck).
2. **Unit Test**: `go test ./... -race`; target coverage minimum 60% untuk paket utilitas/storage.
3. **Integration Test**: gunakan environment staging yang memanggil API internal `POST /sessions` dengan session dummy dan memastikan worker aktif.
4. **Load/Chaos Test**: gunakan `k6` atau tool lain untuk mensimulasikan 100+ session connect/disconnect, memverifikasi orchestrator stabil.
6. **Release Checklist**: update changelog, jalankan migrations via `golang-migrate up`, deploy wizard dan orchestrator secara terpisah.

---
Dokumen ini menjadi panduan awal untuk membangun ulang Little Userbot Maker dengan stack Go + gotd/td, mengikuti prinsip dari project Python.
