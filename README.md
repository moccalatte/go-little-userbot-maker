# Go Little Userbot Maker

Selamat datang di Go Little Userbot Maker! Proyek ini adalah sebuah ekosistem untuk membuat dan mengelola sesi userbot Telegram dengan mudah.

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

2.  **Jalankan Mode Belajar**: Perintah ini akan menjalankan kedua layanan (wizard dan orchestrator) dengan data palsu (mock), sehingga Anda bisa langsung mencoba tanpa setup yang rumit.
    ```bash
    make dev-mock
    ```

3.  **Jalankan Tes (Opsional)**: Untuk memastikan semuanya berfungsi dengan baik, Anda bisa menjalankan tes otomatis.
    ```bash
    go test ./...
    ```

Setelah menjalankan `make dev-mock`, sistem akan aktif dan siap untuk dieksplorasi.

## Dokumentasi Lanjutan

Untuk pemahaman yang lebih mendalam, silakan merujuk ke dokumen berikut:

-   **[Panduan Pengguna (panduan.md)](panduan.md)**: Instruksi lengkap untuk menjalankan dan menguji sistem ini dengan koneksi Telegram dan database sungguhan.
-   **[Arsitektur Proyek (ARCHITECTURE.md)](ARCHITECTURE.md)**: Penjelasan teknis mengenai struktur proyek, lapisan-lapisan arsitektur (delivery, usecase, repository), dan bagaimana semua komponen saling berinteraksi.

Dokumen-dokumen ini dirancang untuk membantu Anda memahami proyek ini dari berbagai tingkat keahlian, mulai dari pengguna non-teknis hingga pengembang perangkat lunak.