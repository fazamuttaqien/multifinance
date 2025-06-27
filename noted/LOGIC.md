Tentu, berikut adalah jawaban sebelumnya yang disajikan dalam format Markdown.

---

# Rancangan Business Logic Layer (BLL)

_Business Logic Layer_ (BLL) adalah lapisan "jantung" dari aplikasi. Lapisan ini bertanggung jawab untuk mengeksekusi semua aturan bisnis, kalkulasi, dan alur kerja (workflow). BLL berada di antara _API Endpoints (Presentation Layer)_ dan _Database (Data Access Layer)_, memastikan data diproses sesuai dengan aturan perusahaan sebelum disimpan atau disajikan.

---

## 🧑‍💼 1. Logika Manajemen Konsumen (Customer Management Logic)

Logika ini mengatur seluruh siklus hidup data konsumen, mulai dari pendaftaran hingga verifikasi.

### a. Service Registrasi Konsumen (Customer Registration Service)

- **Tujuan:** Mengelola pendaftaran konsumen baru secara aman dan memastikan validitas data awal.
- **Input:** Data mentah dari endpoint `POST /api/v1/customers/register`.
- **Aturan & Proses:**
  1.  **Validasi Input:** Memastikan semua data yang diterima sesuai format (misalnya, `nik` harus 16 digit, tanggal lahir adalah tanggal yang valid, dll.).
  2.  **Pemeriksaan Duplikasi:** Melakukan pengecekan ke tabel `customers` untuk memastikan `nik` belum terdaftar. Jika sudah ada, kembalikan error.
  3.  **Pengelolaan File Gambar:** Menerima data gambar (misalnya dalam format Base64), mengubahnya menjadi file, dan menyimpannya ke sistem penyimpanan file (seperti Amazon S3 atau folder lokal).
      > **Praktik Terbaik:** Jangan menyimpan file gambar langsung di database. Simpan URL atau path-nya saja.
  4.  **Pembuatan Record:** Membuat entri baru di tabel `customers` dengan `verification_status` awal diatur ke `'PENDING'`.
  5.  **Pemicu Notifikasi (Trigger):** Setelah berhasil menyimpan, memicu sebuah _event_ untuk memberitahu sistem internal (misalnya, dashboard admin) bahwa ada konsumen baru yang perlu diverifikasi.

### b. Service Verifikasi Konsumen (Customer Verification Service)

- **Tujuan:** Memproses keputusan verifikasi dari tim internal (admin/verifikator).
- **Input:** `customerId` dan hasil verifikasi (`'VERIFIED'` atau `'REJECTED'`) dari admin.
- **Aturan & Proses:**
  1.  **Otorisasi:** Memastikan bahwa pengguna yang memanggil layanan ini memiliki peran (role) yang sesuai, seperti 'Admin' atau 'Verifikator'.
  2.  **Pencarian Konsumen:** Menemukan data konsumen di database berdasarkan `customerId`.
  3.  **Update Status:** Mengubah nilai kolom `verification_status` pada tabel `customers`.
  4.  **Logika Kondisional:**
      - **Jika `'VERIFIED'`:** Memicu _event_ untuk memberitahu tim Analis Kredit bahwa konsumen ini siap untuk penetapan limit.
      - **Jika `'REJECTED'`:** Mencatat alasan penolakan dan secara opsional memicu pengiriman notifikasi (email/push) kepada konsumen.

---

## 💳 2. Logika Manajemen Limit Kredit (Credit Limit Logic)

Logika ini merupakan inti dari bisnis multifinance, yaitu menentukan dan mengelola daya pinjam konsumen.

### a. Service Kalkulasi Limit (Limit Calculation Service)

- **Tujuan:** Menjadi satu-satunya sumber kebenaran (_single source of truth_) untuk mengetahui sisa limit konsumen secara _real-time_.
- **Input:** `customerId` dan `tenor_id`.
- **Aturan & Proses:**
  1.  **Ambil Total Limit:** Mengambil `limit_amount` dari tabel `customer_limits` untuk konsumen dan tenor yang spesifik.
  2.  **Hitung Pemakaian (Used Amount):** Menjumlahkan nilai semua transaksi dari tabel `transactions` yang berstatus `'ACTIVE'` untuk konsumen dan tenor yang sama.
  3.  **Kalkulasi Sisa Limit:** Melakukan perhitungan: `Sisa Limit = Total Limit - Pemakaian`.
- **Output:** Mengembalikan objek yang berisi `total_limit`, `used_amount`, dan `remaining_limit`.

### b. Service Penetapan Limit (Limit Setting Service)

- **Tujuan:** Memungkinkan admin untuk menetapkan atau memperbarui limit kredit konsumen.
- **Input:** `customerId` dan sebuah array yang berisi `{tenor_id, limit_amount}`.
- **Aturan & Proses:**
  1.  **Otorisasi:** Memastikan pengguna memiliki peran sebagai 'Analis Kredit' atau 'Admin'.
  2.  **Operasi `UPSERT`:** Untuk setiap item dalam array input, lakukan operasi "Update or Insert" pada tabel `customer_limits`. Jika kombinasi `customerId` dan `tenor_id` sudah ada, perbarui `limit_amount`-nya. Jika tidak, buat entri baru.
  3.  **Pencatatan Log Audit:** Mencatat setiap perubahan limit untuk keperluan audit dan pelacakan historis.

---

## 🛒 3. Logika Manajemen Transaksi (Transaction Logic)

Ini adalah logika paling kritis karena melibatkan transaksi finansial dan harus menjamin integritas data (prinsip ACID).

### a. Service Pembuatan Transaksi (Transaction Creation Service)

- **Tujuan:** Membuat catatan transaksi baru secara aman, valid, dan **atomik**, sambil memastikan limit tidak terlampaui. Ini adalah tempat di mana **penanganan transaksi bersamaan (concurrency)** diterapkan.
- **Input:** Data transaksi dari partner (`customer_nik`, `tenor_months`, `otr_amount`, dll.).
- **Aturan & Proses:** _(Harus dijalankan dalam satu blok Transaksi Database)_
  1.  **Mulai Transaksi:** Memulai blok transaksi database (`BEGIN TRANSACTION`).
  2.  **Kunci Data Konsumen (`Pessimistic Locking`):** Lakukan `SELECT ... FOR UPDATE` pada baris konsumen yang relevan. Ini akan **mengunci baris tersebut** sehingga tidak ada proses lain yang bisa membacanya (dengan `FOR UPDATE`) atau mengubahnya sampai transaksi ini selesai.
  3.  **Validasi Ulang Limit:** **Di dalam transaksi yang terkunci ini**, panggil `Limit Calculation Service` untuk mendapatkan sisa limit yang paling akurat dan terkini. _Ini adalah langkah pengamanan terpenting untuk mencegah **race condition**._
  4.  **Bandingkan Limit:** Bandingkan `remaining_limit` dengan `transaction_amount` yang diajukan.
      - **Jika tidak cukup:** Batalkan proses, jalankan `ROLLBACK` untuk melepaskan kunci, dan kembalikan pesan error "Insufficient Limit".
  5.  **Jika cukup, lanjutkan:**
      - **Generate Nomor Kontrak:** Buat nomor kontrak unik sesuai aturan bisnis.
      - **Hitung Komponen Finansial:** Berdasarkan `otr_amount`, `admin_fee`, dan suku bunga yang berlaku, hitung `total_interest` dan `total_installment_amount`.
      - **Buat Catatan Transaksi:** Lakukan `INSERT` ke tabel `transactions` dengan semua data yang sudah divalidasi dan dihitung, dengan `status` awal `'APPROVED'` atau `'ACTIVE'`.
  6.  **Simpan Perubahan:** Jika semua langkah di atas berhasil, jalankan `COMMIT` untuk menyimpan semua perubahan ke database secara permanen. Kunci pada baris konsumen akan otomatis dilepaskan.
  7.  **Pemicu Pasca-Transaksi:** Setelah `COMMIT` berhasil, picu _event_ lain seperti mengirim email konfirmasi kontrak ke konsumen atau notifikasi ke sistem akunting.
