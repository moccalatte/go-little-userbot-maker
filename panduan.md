# Panduan Penggunaan & Pengujian (Non-Teknis)

Dokumen ini membantu operator/staf non-teknis menjalankan Little Userbot Maker berbasis Go.

## 1. Persiapan Awal
1. **Install perangkat lunak**
   - [Docker Desktop](https://www.docker.com/) atau Docker Engine di VPS.
   - Git (opsional bila ingin menarik kode terbaru).
2. **Salin berkas contoh konfigurasi**
   ```bash
   cp .env.wizard.example .env.wizard
   cp .env.userbot.example .env.userbot
   ```
3. **Isi nilai penting** pada `.env.wizard` & `.env.userbot`:
   - `WIZARD_BOT_TOKEN`: token bot Telegram dari @BotFather.
   - `ORCH_SECRET_KEY`: kata rahasia minimal 32 karakter (bebas, tapi simpan aman).
   - `DB_URL`: alamat PostgreSQL (pakai default bila menjalankan via Docker Compose lokal).

## 2. Menjalankan Seluruh Sistem Sekaligus
### Opsi A – Mode belajar (mock, tanpa Postgres & token bot)
```bash
make dev-mock
```
Perintah ini menyalakan wizard mock dan orchestrator in-memory sehingga aman untuk eksplorasi awal.

### Opsi B – Skrip satu perintah (stack penuh)
```bash
./run_local.sh
```
Perintah ini akan:
1. Memeriksa berkas `.env`.
2. Menyalakan Bot Wizard, Orchestrator, PostgreSQL, dan Redis.

### Opsi C – Mode daemon (latar belakang)
```bash
make compose-up-local
```
Untuk mematikan:
```bash
make compose-down-local
```

## 3. Menghubungkan Telegram Bot
1. Buka aplikasi Telegram, cari bot Anda, lalu tekan **Start**.
2. Menu utama menyediakan tombol:
   - **🤖 Buat Userbot**: panduan login OTP/QR.
   - **🔑 Token Login**: tempel session string panjang.
   - **⚙️ Kelola Userbot**: konfigurasi Reply Guard, Broadcast, dll.
   - **🔧 Admin Settings**: hanya muncul jika ID Anda tercantum pada `WIZARD_ADMIN_IDS`.

### Contoh flow OTP singkat
1. Pilih **🤖 Buat Userbot** → **📱 OTP**.
2. Masukkan nomor telepon lengkap (format `+62...`).
3. Isi `API ID` & `API hash` dari https://my.telegram.org.
4. Ketik kode OTP dan password 2FA (jika ada, atau masukkan `-`). Progres tersimpan otomatis di database (`wizard_runs`), jadi aman bila harus mengulang.
5. Wizard mengirim session lengkap ke Orchestrator, mencatat audit, dan menyiapkan perintah dasar (`/ping`, `/help`) untuk userbot baru. Anda menerima konfirmasi keberhasilan.

## 4. Pemeriksaan & Pengujian
- **Tes otomatis**: jalankan `go test ./...` (untuk tim teknis).
- **Cek kesehatan backend**: `curl http://localhost:8080/healthz` (OK jika membaca `ok`).
- **Log percakapan wizard**: folder `storage/logs/wizard/<telegram_id>/<tanggal>.jsonl`.
- **Log orchestrator**: melalui Docker (`docker logs <container_orchestrator>`) atau volume `userbot_logs`.
- **Database**: jalankan `./scripts/migrate.sh` saat pertama kali menyalakan atau setelah update skema.

## 5. Troubleshooting Singkat
| Gejala | Langkah cepat |
| --- | --- |
| Bot tidak merespons | Pastikan container `wizard` aktif (`docker ps`). Cek token di `.env.wizard`.
| Userbot tidak berjalan | Lihat log orchestrator. Pastikan `ORCH_SECRET_KEY` terisi; bila masih tahap belajar pastikan `ORCH_ENABLE_MOCK=true`.
| OTP gagal | Verifikasi nomor telepon & API ID/Hash. Pastikan Telegram tidak menahan login (tunggu beberapa menit).
| Token login ditolak | Pastikan teks lebih dari 100 karakter dan benar-benar base64 MTProto.

## 6. Hentikan & Rollback
1. `make compose-down-local` untuk mematikan seluruh layanan.
2. Kembalikan backup volume `userbot_db_data` jika diperlukan.
3. Jalankan kembali layanan versi sebelumnya (misal image lama) lalu cek `/healthz` hingga status OK.

Selesai! Sistem siap dipakai kembali oleh tim maupun asisten AI untuk mengelola sesi userbot Telegram.
