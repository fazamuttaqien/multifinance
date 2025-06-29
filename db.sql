-- #####################################################################
-- # Skrip SQL untuk Database PT XYZ Multifinance
-- #####################################################################

CREATE DATABASE IF NOT EXISTS loan_system;

USE loan_system;

DROP TABLE IF EXISTS `transactions`;
DROP TABLE IF EXISTS `customer_limits`;
DROP TABLE IF EXISTS `tenors`;
DROP TABLE IF EXISTS `customers`;


-- #####################################################################
-- # Tabel 1: customers
-- # Menyimpan semua data personal pengguna, kredensial, dan status.
-- #####################################################################
CREATE TABLE `customers` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `nik` VARCHAR(16) NOT NULL,
  `full_name` VARCHAR(255) NOT NULL,
  `legal_name` VARCHAR(255) NOT NULL,
  `birth_place` VARCHAR(100) NOT NULL,
  `birth_date` DATE NOT NULL,
  `salary` DECIMAL(15,2) NOT NULL,
  `password` VARCHAR(255) NOT NULL COMMENT 'Menyimpan password yang sudah di-hash (bcrypt)',
  `ktp_photo_url` VARCHAR(255) NOT NULL,
  `selfie_photo_url` VARCHAR(255) NOT NULL,
  `role` ENUM('admin', 'customer', 'partner') NOT NULL DEFAULT 'customer',
  `verification_status` ENUM('PENDING', 'VERIFIED', 'REJECTED') NOT NULL DEFAULT 'PENDING',
  `created_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_customers_nik` (`nik`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- #####################################################################
-- # Tabel 2: tenors
-- # Tabel referensi (lookup table) untuk menyimpan pilihan tenor.
-- #####################################################################
CREATE TABLE `tenors` (
  `id` INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `duration_months` TINYINT UNSIGNED NOT NULL,
  `description` VARCHAR(50) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_tenors_duration_months` (`duration_months`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Memasukkan data awal untuk tenor
INSERT INTO `tenors` (`duration_months`, `description`) VALUES
(1, '1 Bulan'),
(2, '2 Bulan'),
(3, '3 Bulan'),
(6, '6 Bulan'),
(12, '12 Bulan');


-- #####################################################################
-- # Tabel 3: customer_limits
-- # Tabel junction untuk menyimpan limit spesifik per customer per tenor.
-- #####################################################################
CREATE TABLE `customer_limits` (
  `customer_id` BIGINT UNSIGNED NOT NULL,
  `tenor_id` INT UNSIGNED NOT NULL,
  `limit_amount` DECIMAL(15,2) NOT NULL,
  PRIMARY KEY (`customer_id`, `tenor_id`),
  CONSTRAINT `fk_customer_limits_customer`
    FOREIGN KEY (`customer_id`)
    REFERENCES `customers` (`id`)
    ON DELETE CASCADE,
  CONSTRAINT `fk_customer_limits_tenor`
    FOREIGN KEY (`tenor_id`)
    REFERENCES `tenors` (`id`)
    ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- #####################################################################
-- # Tabel 4: transactions
-- # Mencatat semua transaksi pembiayaan yang berhasil dilakukan.
-- #####################################################################
CREATE TABLE `transactions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `contract_number` VARCHAR(50) NOT NULL,
  `customer_id` BIGINT UNSIGNED NOT NULL,
  `tenor_id` INT UNSIGNED NOT NULL,
  `asset_name` VARCHAR(255) NOT NULL,
  `otr_amount` DECIMAL(15,2) NOT NULL COMMENT 'On The Road Price',
  `admin_fee` DECIMAL(15,2) NOT NULL,
  `total_interest` DECIMAL(15,2) NOT NULL,
  `total_installment_amount` DECIMAL(15,2) NOT NULL COMMENT 'Total pinjaman pokok + bunga',
  `status` ENUM('PENDING', 'APPROVED', 'ACTIVE', 'PAID_OFF', 'CANCELLED') NOT NULL DEFAULT 'PENDING',
  `transaction_date` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_transactions_contract_number` (`contract_number`),
  CONSTRAINT `fk_transactions_customer`
    FOREIGN KEY (`customer_id`)
    REFERENCES `customers` (`id`)
    ON DELETE RESTRICT,
  CONSTRAINT `fk_transactions_tenor`
    FOREIGN KEY (`tenor_id`)
    REFERENCES `tenors` (`id`)
    ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;