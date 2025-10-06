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
- **Shared Services**: logging berbasis file yang konsisten dan tracing (opsional).

### 2.1 Deployment Model & Directory Layout
- **Folder Top-Level** (`/home/app/go-little-userbot-maker` contoh):
  - `services/bot-wizard/` → Kode sumber untuk layanan Bot Wizard.
  - `services/userbot-orchestrator/` → Kode sumber untuk layanan Userbot Orchestrator.
  - `pkg/` → Pustaka bersama yang digunakan oleh beberapa layanan (misalnya, `logger`, `config`).
  - `docs/` → Semua file dokumentasi proyek.
  - `scripts/` → Skrip bantuan untuk development.
  - `docker-compose.local.yml` → Menjalankan semua layanan secara lokal untuk pengujian.
  - `docker-compose.wizard.yml` → Stack produksi untuk VPS A (hanya bot wizard).
  - `docker-compose.userbot.yml` → Stack produksi untuk VPS B (orchestrator + PostgreSQL).
  - `Makefile` → Perintah sederhana untuk build, test, dan menjalankan layanan.
- **Volume Data**: Direktori data PostgreSQL tidak disimpan di repo; gunakan volume bernama (`userbot_db_data`) agar mudah dipindah dan di-backup.
- **Environment Files**: Simpan `.env.wizard` dan `.env.userbot` terpisah; setiap compose file memanggil env masing-masing.

### 2.2 Workflow Pengembangan
- **Beginner Mock Mode**: Jalankan `make dev-mock` untuk menyalakan orchestrator in-memory + wizard mock. Tidak perlu Postgres atau bot token; seluruh log tetap tercatat di terminal dan di direktori `./logs`.
- **Local Testing Penuh**: Jalankan `docker compose -f docker-compose.local.yml up --build` untuk menjalankan wizard, orchestrator, dan Postgres di mesin lokal.
- **CI/CD-friendly**: Setiap image dibangun terpisah (`bot-wizard:latest`, `userbot-orchestrator:latest`).

### 2.3 Logging & Audit Topology
- **Tujuan**: Seluruh interaksi (terminal, percakapan wizard, aksi admin, aktivitas userbot) terekam dengan struktur konsisten agar mudah dibaca oleh developer dan AI untuk debugging.
- **Sistem Logging**: Proyek ini menggunakan logger berbasis file sederhana yang ada di `pkg/logger`.
  - **Lokasi File**: Log disimpan di `./logs/{nama-layanan}/{id-telegram}/{tanggal}.log`.
    - Untuk log yang berlaku untuk seluruh layanan, `id-telegram` akan menjadi `main`.
    - Contoh: `/logs/userbot-orchestrator/main/2025-10-05.log` atau `/logs/userbot-worker/123456789/2025-10-05.log`.
  - **Format Log**: `[timestamp] [level] [service] [telegram_id] message`
    - Contoh: `[2025-10-05T14:35:08Z] [INFO] [userbot-orchestrator] [main] starting service`
  - **Rotasi & Retensi**: Log dirotasi setiap hari. File log yang lebih lama dari 7 hari secara otomatis dihapus untuk menghemat ruang.
- **Akses Log**: Developer atau AI dapat langsung membaca file-file ini untuk menganalisis perilaku aplikasi, melacak kesalahan, atau memantau aktivitas pengguna tertentu. Semua `stdout` dan `stderr` dari aplikasi juga dialihkan ke file-file log ini.

## 3. Bot Wizard Requirements (Go)
-### 3.1 Conversational Flow
- Library: `github.com/go-telegram-bot-api/telegram-bot-api/v5`.
- Semua menu menggunakan ReplyKeyboardMarkup/InlineKeyboardMarkup.
- State machine disimpan dalam map in-memory dengan TTL, di-keyed oleh chat_id.
- **Logging**: Semua interaksi, kesalahan, dan perubahan state dicatat menggunakan logger `pkg/logger` ke file log yang sesuai. Ini menggantikan sistem transkrip yang terpisah.

#### Menu & Flow
(Flow tetap sama seperti sebelumnya: Buat Userbot, Token Login, Kelola Userbot, dll.)

### 3.2 Error Handling
- Gunakan context dengan timeout untuk setiap panggilan API eksternal.
- Panic harus dilindungi `recover` dan dicatat ke file log sebagai `ERROR`.

## 4. Userbot Orchestrator (GoTD)
### 4.1 Session Management
- Library: `github.com/gotd/td`.
- Setiap sesi dijalankan dalam goroutine terisolasi (`worker`).
- Session string disimpan terenkripsi di PostgreSQL.
- Worker diidentifikasi dengan `telegram_id`. Saat layanan dimulai, semua sesi aktif dari database akan di-bootstrap.
- Setiap worker memiliki file lognya sendiri (`/logs/userbot-worker/{telegram_id}/{date}.log`).

### 4.2 Command Framework
- Standardize interface `Command` dengan metode `Name()`, `Execute()`, `Help()`.
- Semua eksekusi command harus dicatat ke log, termasuk argumen yang diberikan.

### 4.3 Monitoring & Telemetry
- **Logging**: Monitoring utama dilakukan melalui file log. Tidak ada framework metrik yang kompleks.
- **Health Endpoint**: Endpoint `/healthz` memeriksa konektivitas database.

## 5. Data Model (PostgreSQL)
(Model data tetap sama: `users`, `sessions`, `reply_guard_rules`, dll.)

### 5.1 ERD Basis Data
```
┌────────────┐        ┌──────────────┐
│   users    │1     * │   sessions   │
│------------│◄──────┤--------------│
│ id (PK)    │        │ id (PK)      │
│ telegram_id│        │ user_id (FK) │
│ username   │        │ telegram_id  │
│ plan       │        │ status       │
└────────────┘        └──────────────┘
        │1                     │*
        │                      │
        │        ┌──────────────────────────┐
        ├────────┤ reply_guard_rules        │
        │        │ id (PK)                  │
        │        │ user_id (FK)             │
        │        │ status                   │
        │        └──────────────────────────┘
        │
        │        ┌──────────────────────────┐
        ├────────┤ broadcast_jobs           │
        │        │ id (PK)                  │
        │        │ user_id (FK)             │
        │        │ enabled                  │
        │        └──────────────────────────┘
        │
        │        ┌──────────────────────────┐
        └────────┤ usage_stats              │
                 │ id (PK)                  │
                 │ user_id (FK)             │
                 │ command / count          │
                 └──────────────────────────┘
```

**Catatan Relasi**
- `users` memegang identitas utama setiap pemilik userbot (`telegram_id` unik) dan menjadi sumber FK untuk tabel lain.
- `sessions` menyimpan session string terenkripsi per userbot; `user_id` opsional agar bisa menyimpan sesi sementara sebelum profil pengguna dibuat.
- `reply_guard_rules`, `broadcast_jobs`, dan `usage_stats` bergantung pada `users` dengan relasi one-to-many dan akan ikut terhapus saat user dihapus (cascade pada sebagian tabel).
- `sessions.session_hash`, `broadcast_jobs.targets_json`, serta kolom JSON/metadata lain dipakai orchestrator untuk otomasi lanjutan tanpa menambah tabel baru.

### 5.2 Reliability Practices (Go-centric)
- **Logging**: Gunakan logger `pkg/logger` secara konsisten di seluruh aplikasi. Pastikan untuk mencatat semua kesalahan, peringatan, dan alur kontrol penting.
- **Context Propagation**: Teruskan `context.Context` di semua pemanggilan fungsi untuk timeout dan pembatalan yang benar.
- **Error Handling**: Gunakan error wrapping (`fmt.Errorf("...: %w", err)`) agar mudah di-trace melalui log.
- **Panic & Leak Detection**: Bungkus goroutine dengan `defer` recovery yang mencatat panic ke log.

## 6. Future Enhancements
(Dihapus: Referensi ke metrik, Grafana, Sentry, dll.)

## 7. Delivery Checklist
- [ ] Environment vars diatur di kedua layanan.
- [ ] `make dev-mock` berjalan lancar dan menulis log ke direktori `./logs`.
- [ ] Unit test untuk flow inti.
- [ ] CI pipeline: lint, test, build.

## 8. Risks & Mitigations
(Risiko tetap sama: rate limits, session leakage, dll.)

## 9. Implementation Guidelines (Untuk AI/Developer)
- **Bahasa & Versi**: Go 1.22.x.
- **Dependensi Utama**:
  - Bot wizard: `go-telegram-bot-api/v5`.
  - Userbot: `github.com/gotd/td`.
- **Project Layout**:
  - `services/bot-wizard/`: Logika untuk layanan bot wizard.
  - `services/userbot-orchestrator/`: Logika untuk layanan orchestrator.
  - `pkg/`: Pustaka bersama (`logger`, `config`, `storage`).
- **Error Handling**: Bungkus error dan catat dengan logger.
- **Logging**: Gunakan instance `pkg/logger.Logger` yang diteruskan melalui dependency injection. Jangan membuat instance logger baru secara manual di dalam fungsi.
- **Testing**: Jalankan `go test ./...`.

## 10. Development Workflow & QA
1. **Linting**: `golangci-lint run`.
2. **Unit Test**: `go test ./... -race`.
3. **Integration Test**: Gunakan `make dev-mock` untuk memverifikasi bahwa kedua layanan dapat berjalan bersama dan log dibuat dengan benar.
4. **Release Checklist**: Update changelog, jalankan migrasi database, deploy layanan.

---
Dokumen ini menjadi panduan awal untuk membangun ulang Little Userbot Maker dengan stack Go + gotd/td, mengikuti prinsip arsitektur yang bersih dan modular.
