# PT XYZ Multifinance System Overview

Sistem yang ada di PT XYZ Multifinance merupakan web-based monolith pada sisi customer facing, menggunakan database MySQL yang disimpan dalam Virtual Machine on-premise server. Aplikasi dan database berada dalam satu instance yang sama.

---

## Gambaran Sistem

### Limit Pengajuan Peminjaman Konsumen

Setiap konsumen PT XYZ memiliki limit untuk melakukan pengajuan peminjaman, dengan limit berbeda untuk tenor 1, 2, 3, dan 6 bulan.

| Konsumen | 1         | 2         | 3         | 6         |
|----------|-----------|-----------|-----------|-----------|
| Budi     | 100.000   | 200.000   | 500.000   | 700.000   |
| Annisa   | 1.000.000 | 1.200.000 | 1.500.000 | 2.000.000 |

Konsumen dapat menghabiskan limitnya dalam beberapa transaksi di PT XYZ, ecommerce, maupun dealer konvensional.

---

### Data Personal Konsumen

Setiap konsumen memiliki data personal sebagai berikut:

| Data           | Keterangan                                 |
|----------------|--------------------------------------------|
| NIK            | Nomor KTP Konsumen                         |
| Full name      | Nama Lengkap Konsumen                      |
| Legal name     | Nama Konsumen di KTP                       |
| Tempat Lahir   | Tempat lahir konsumen sesuai KTP           |
| Tanggal Lahir  | Tanggal lahir konsumen sesuai KTP          |
| Gaji           | Gaji Konsumen                              |
| Foto KTP       | Foto KTP Konsumen                          |
| Foto Selfie    | Foto Selfie Konsumen                       |

---

### Data Transaksi Konsumen

Setiap transaksi konsumen, baik di ecommerce, web PT XYZ, maupun dealer yang bekerja sama, akan direkam dalam database dengan data sebagai berikut:

| Data            | Keterangan                                                                                   |
|-----------------|---------------------------------------------------------------------------------------------|
| Nomor Kontrak   | Nomor kontrak untuk setiap transaksi konsumen                                               |
| OTR             | Nilai On The Road barang (White Goods, Motor, atau Mobil)                                   |
| Admin Fee       | Biaya administrasi transaksi barang (White Goods, Motor, atau Mobil)                        |
| Jumlah Cicilan  | Total cicilan transaksi barang (White Goods, Motor, atau Mobil)                             |
| Jumlah Bunga    | Total bunga yang ditagihkan pada setiap transaksi barang (White Goods, Motor, atau Mobil)   |
| Nama Asset      | Nama aset yang dibeli konsumen                                                              |

---

## Business Requirement

- Mampu melayani 99,9% availability customer facing system
- Mampu menyediakan proactive action dan keterbukaan terhadap error, bug, performance, dan penggunaan
- Mampu menyediakan deployment yang cepat dan akurat terhadap suatu feature / bug
- Mampu menyediakan keamanan terhadap aplikasi yang berbasis pada standar OWASP
- Mampu menyediakan data yang ACID (Atomicity, Consistency, Isolation,