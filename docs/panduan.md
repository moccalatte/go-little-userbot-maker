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
Perintah ini menyalakan wizard mock dan orchestrator in-memory sehingga aman untuk eksplorasi awal. Log akan muncul di terminal dan juga disimpan di direktori `./logs`.

### Opsi B – Skrip satu perintah (stack penuh)
```bash
./run_local.sh
```
Perintah ini akan:
1. Memeriksa berkas `.env`.
2. Menyalakan Bot Wizard, Orchestrator, dan PostgreSQL.

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
   - **⚙️ Kelola Userbot**: konfigurasi fitur userbot.
   - **🔧 Admin Settings**: hanya muncul jika ID Anda tercantum pada `WIZARD_ADMIN_IDS`.

### Contoh flow OTP singkat
1. Pilih **🤖 Buat Userbot** → **📱 OTP**.
2. Masukkan nomor telepon lengkap (format `+62...`).
3. Isi `API ID` & `API hash` dari https://my.telegram.org.
4. Ketik kode OTP dan password 2FA (jika ada, atau masukkan `-`).
5. Wizard mengirim session ke Orchestrator; Anda menerima konfirmasi keberhasilan.

## 4. Pemeriksaan & Pengujian
- **Tes otomatis**: jalankan `go test ./...` (untuk tim teknis).
- **Cek kesehatan backend**: `curl http://localhost:8080/healthz` (OK jika membaca `ok`).
- **Cek Log**: Semua log sekarang disimpan dalam file teks yang mudah dibaca di direktori `./logs`.
  - **Struktur**: `./logs/{nama-layanan}/{id-pengguna-atau-main}/{tanggal}.log`
  - **Contoh Log Wizard**: `logs/bot-wizard/main/2025-10-05.log`
  - **Contoh Log Orchestrator**: `logs/userbot-orchestrator/main/2025-10-05.log`

## 5. Troubleshooting Singkat
| Gejala | Langkah cepat |
| --- | --- |
| Bot tidak merespons | Pastikan container `wizard` aktif (`docker ps`). Cek token di `.env.wizard`. Lihat log di `logs/bot-wizard/main/` untuk pesan error.
| Userbot tidak berjalan | Lihat log di `logs/userbot-orchestrator/main/`. Pastikan `ORCH_SECRET_KEY` terisi.
| OTP gagal | Verifikasi nomor telepon & API ID/Hash. Cek log untuk melihat detail error dari proses login.
| Token login ditolak | Pastikan teks lebih dari 100 karakter dan formatnya benar. Cek log untuk pesan error spesifik.

## 6. Hentikan & Rollback
1. `make compose-down-local` untuk mematikan seluruh layanan.
2. Kembalikan backup volume `userbot_db_data` jika diperlukan.
3. Jalankan kembali layanan versi sebelumnya (misal image lama) lalu cek `/healthz` hingga status OK.

Selesai! Sistem siap dipakai kembali oleh tim maupun asisten AI untuk mengelola sesi userbot Telegram.