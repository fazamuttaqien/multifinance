### Endpoint API yang Diperlukan

Berdasarkan fungsionalitas yang telah dibahas, kita bisa merancang beberapa endpoint RESTful API. Endpoint ini dapat dibagi menjadi dua kategori utama: **Internal** (untuk admin/karyawan) dan **Eksternal** (untuk konsumen dan partner/pihak ketiga).

#### A. Endpoint Eksternal (Public/Customer & Partner Facing)

Endpoint ini harus aman, cepat, dan memiliki dokumentasi yang jelas.

##### Untuk Konsumen (Customer Endpoints)
*Akses: Membutuhkan autentikasi (misal, JWT Token) setelah login.*

1.  **Registrasi Konsumen Baru**
    *   `POST /api/v1/customers/register`
    *   **Request Body:** `{ "nik": "...", "full_name": "...", "legal_name": "...", "birth_place": "...", "birth_date": "...", "salary": ..., "ktp_photo": "...", "selfie_photo": "..." }`
    *   **Response:** `201 Created` - `{ "message": "Registration successful, waiting for verification." }`

2.  **Melihat Profil Sendiri**
    *   `GET /api/v1/me/profile`
    *   **Response:** `200 OK` - `{ "id": ..., "nik": "...", "full_name": "...", ... }`

3.  **Mengupdate Profil Sendiri**
    *   `PUT /api/v1/me/profile`
    *   **Request Body:** `{ "full_name": "...", "salary": ... }` (hanya field yang bisa diubah)
    *   **Response:** `200 OK` - `{ "message": "Profile updated." }`

4.  **Melihat Limit Peminjaman Sendiri**
    *   `GET /api/v1/me/limits`
    *   **Response:** `200 OK` - `[ { "tenor_months": 1, "limit_amount": 100000, "used_amount": 0, "remaining_limit": 100000 }, { "tenor_months": 3, ... } ]`
        * *Catatan: `used_amount` dan `remaining_limit` dihitung secara dinamis (on-the-fly) oleh backend.*

5.  **Melihat Riwayat Transaksi Sendiri**
    *   `GET /api/v1/me/transactions`
    *   **Query Params (Opsional):** `?status=ACTIVE&page=1&limit=10`
    *   **Response:** `200 OK` - `{ "data": [ { "contract_number": "...", "asset_name": "...", "total_installment_amount": ..., "status": "ACTIVE" }, ... ], "pagination": { ... } }`

##### Untuk Partner (Ecommerce/Dealer Endpoints)
*Akses: Membutuhkan autentikasi berbasis API Key atau OAuth2 Client Credentials.*

6.  **Mengecek Ketersediaan Limit Konsumen**
    *   `POST /api/v1/partners/check-limit`
    *   **Request Body:** `{ "customer_nik": "...", "tenor_months": 3, "transaction_amount": 500000 }`
    *   **Response:**
        *   `200 OK` - `{ "status": "approved", "message": "Limit is sufficient." }`
        *   `422 Unprocessable Entity` - `{ "status": "rejected", "reason": "insufficient_limit" }`

7.  **Membuat Transaksi Baru**
    *   `POST /api/v1/partners/transactions`
    *   **Request Body:** `{ "customer_nik": "...", "tenor_months": 3, "asset_name": "...", "otr_amount": ..., "admin_fee": ... }`
    *   **Response:** `201 Created` - `{ "contract_number": "XYZ-2023-12345", "status": "APPROVED", ... }`

#### B. Endpoint Internal (Admin Facing)

Endpoint ini digunakan oleh karyawan PT XYZ melalui aplikasi back-office/dashboard admin.

*Akses: Membutuhkan autentikasi dan otorisasi berbasis peran (Role-Based Access Control - RBAC).*

8.  **Mendapatkan Daftar Konsumen (untuk Verifikasi, dll)**
    *   `GET /api/v1/admin/customers`
    *   **Query Params:** `?status=PENDING&sort_by=created_at&page=1`
    *   **Response:** `200 OK` - `{ "data": [ ... ], "pagination": { ... } }`

9.  **Melihat Detail Satu Konsumen**
    *   `GET /api/v1/admin/customers/{customerId}`
    *   **Response:** `200 OK` - `{ "id": ..., "nik": "...", "ktp_photo_url": "...", "selfie_photo_url": "...", ... }`

10. **Memverifikasi atau Menolak Konsumen**
    *   `POST /api/v1/admin/customers/{customerId}/verify`
    *   **Request Body:** `{ "status": "VERIFIED" }` atau `{ "status": "REJECTED", "reason": "Foto KTP tidak jelas" }`
    *   **Response:** `200 OK` - `{ "message": "Customer status updated." }`

11. **Menetapkan atau Mengupdate Limit Konsumen**
    *   `POST /api/v1/admin/customers/{customerId}/limits`
    *   **Request Body:** `[ { "tenor_months": 1, "limit_amount": 100000 }, { "tenor_months": 3, "limit_amount": 500000 } ]`
    *   **Response:** `200 OK` - `{ "message": "Customer limits set successfully." }`

---