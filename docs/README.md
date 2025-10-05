# Go Little Userbot Maker

Selamat datang di Go Little Userbot Maker! Proyek ini adalah sebuah ekosistem untuk membuat dan mengelola sesi userbot Telegram dengan mudah, dibangun dengan arsitektur yang bersih dan modular.

## Tujuan Proyek

Tujuan utama proyek ini adalah menyediakan dua layanan utama:

1.  **Bot Wizard**: Sebuah bot Telegram yang memandu pengguna melalui proses login (via OTP atau QR code) untuk membuat sesi userbot baru.
2.  **Userbot Orchestrator**: Layanan backend yang aman untuk menyimpan dan mengelola sesi-sesi userbot tersebut.

## Memulai (Getting Started)

Cara termudah untuk mencoba proyek ini adalah dengan menjalankan "mode belajar" yang tidak memerlukan koneksi ke Telegram atau database.

### Persyaratan

-   Go (versi 1.22 atau lebih baru)
-   Docker & Docker Compose

### Langkah Cepat

1.  **Siapkan Konfigurasi**: Salin file contoh `.env` untuk memulai. Anda tidak perlu mengubah isinya untuk mode belajar.
    ```bash
    cp .env.wizard.example .env.wizard
    cp .env.userbot.example .env.userbot
    ```

2.  **Jalankan Mode Belajar**: Perintah ini akan menjalankan kedua layanan (wizard dan orchestrator) dengan data palsu (mock).
    ```bash
    make dev-mock
    ```
    Setelah menjalankan perintah ini, sistem akan aktif. Anda akan melihat output log di terminal, dan log file juga akan dibuat di direktori `./logs`.

3.  **Jalankan Tes (Opsional)**: Untuk memastikan semuanya berfungsi dengan baik, Anda bisa menjalankan tes otomatis.
    ```bash
    go test ./...
    ```

## Logging

Proyek ini menggunakan sistem logging berbasis file yang sederhana dan konsisten. Semua output dari terminal dan proses latar belakang disimpan dalam format yang mudah dibaca.

-   **Lokasi Log**: `./logs/{nama-layanan}/{id-telegram}/{tanggal}.log`
-   **Contoh**: `./logs/userbot-orchestrator/main/2025-10-05.log`

Sistem logging ini dirancang untuk memudahkan debugging, baik oleh developer maupun oleh asisten AI. Untuk detail lebih lanjut, lihat dokumen arsitektur.

## Dokumentasi Lanjutan

Untuk pemahaman yang lebih mendalam, silakan merujuk ke dokumen berikut:

-   **[Panduan Pengguna (panduan.md)](./panduan.md)**: Instruksi lengkap untuk menjalankan dan menguji sistem ini dengan koneksi Telegram dan database sungguhan.
-   **[Arsitektur Proyek (ARCHITECTURE.md)](./ARCHITECTURE.md)**: Penjelasan teknis mengenai struktur proyek, lapisan-lapisan arsitektur, dan bagaimana semua komponen saling berinteraksi.
-   **[Product Requirements (go_prd.md)](./go_prd.md)**: Dokumen detail yang menjelaskan persyaratan produk dan fitur.

Dokumen-dokumen ini dirancang untuk membantu Anda memahami proyek ini dari berbagai tingkat keahlian, mulai dari pengguna non-teknis hingga pengembang perangkat lunak.