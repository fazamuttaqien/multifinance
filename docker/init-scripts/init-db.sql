-- init-db.sql
-- This script will be executed when MySQL container starts for the first time

-- Create additional databases if needed
CREATE DATABASE IF NOT EXISTS loan_system_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS loan_system_staging CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Create additional users
CREATE USER IF NOT EXISTS 'app_user'@'%' IDENTIFIED BY 'app_password123';
CREATE USER IF NOT EXISTS 'readonly_user'@'%' IDENTIFIED BY 'readonly_password123';

-- Grant privileges
GRANT ALL PRIVILEGES ON loan_system.* TO 'loan_user'@'%';
GRANT ALL PRIVILEGES ON loan_system.* TO 'app_user'@'%';
GRANT ALL PRIVILEGES ON loan_system_test.* TO 'app_user'@'%';
GRANT ALL PRIVILEGES ON loan_system_staging.* TO 'app_user'@'%';

-- Grant read-only privileges
GRANT SELECT ON loan_system.* TO 'readonly_user'@'%';

-- Flush privileges
FLUSH PRIVILEGES;

-- Use the main database
USE loan_system;

-- Create customers table
CREATE TABLE IF NOT EXISTS customers (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    nik VARCHAR(16) NOT NULL UNIQUE,
    full_name VARCHAR(255) NOT NULL,
    legal_name VARCHAR(255) NOT NULL,
    birth_place VARCHAR(100) NOT NULL,
    birth_date DATE NOT NULL,
    salary DECIMAL(15, 2) NOT NULL,
    ktp_photo_url VARCHAR(255) NOT NULL,
    selfie_photo_url VARCHAR(255) NOT NULL,
    verification_status ENUM('PENDING', 'VERIFIED', 'REJECTED') NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_customers_nik (nik),
    INDEX idx_customers_verification_status (verification_status),
    INDEX idx_customers_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create tenors table
CREATE TABLE IF NOT EXISTS tenors (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    duration_months TINYINT UNSIGNED NOT NULL UNIQUE,
    description VARCHAR(50),
    
    -- Indexes
    INDEX idx_tenors_duration (duration_months)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create customer_limits table
CREATE TABLE IF NOT EXISTS customer_limits (
    customer_id BIGINT UNSIGNED NOT NULL,
    tenor_id INT UNSIGNED NOT NULL,
    limit_amount DECIMAL(15, 2) NOT NULL,
    
    PRIMARY KEY (customer_id, tenor_id),
    
    -- Foreign Keys
    CONSTRAINT fk_customer_limits_customer_id 
        FOREIGN KEY (customer_id) REFERENCES customers(id) 
        ON DELETE CASCADE,
    CONSTRAINT fk_customer_limits_tenor_id 
        FOREIGN KEY (tenor_id) REFERENCES tenors(id) 
        ON DELETE RESTRICT,
        
    -- Indexes
    INDEX idx_customer_limits_customer_id (customer_id),
    INDEX idx_customer_limits_tenor_id (tenor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    contract_number VARCHAR(50) NOT NULL UNIQUE,
    customer_id BIGINT UNSIGNED NOT NULL,
    tenor_id INT UNSIGNED NOT NULL,
    asset_name VARCHAR(255) NOT NULL,
    otr_amount DECIMAL(15, 2) NOT NULL,
    admin_fee DECIMAL(15, 2) NOT NULL,
    total_interest DECIMAL(15, 2) NOT NULL,
    total_installment_amount DECIMAL(15, 2) NOT NULL,
    status ENUM('PENDING', 'APPROVED', 'ACTIVE', 'PAID_OFF', 'CANCELLED') NOT NULL DEFAULT 'PENDING',
    transaction_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Foreign Keys
    CONSTRAINT fk_transactions_customer_id 
        FOREIGN KEY (customer_id) REFERENCES customers(id) 
        ON DELETE RESTRICT,
    CONSTRAINT fk_transactions_tenor_id 
        FOREIGN KEY (tenor_id) REFERENCES tenors(id) 
        ON DELETE RESTRICT,
        
    -- Indexes
    INDEX idx_transactions_customer_id (customer_id),
    INDEX idx_transactions_contract_number (contract_number),
    INDEX idx_transactions_status (status),
    INDEX idx_transactions_transaction_date (transaction_date),
    INDEX idx_transactions_tenor_id (tenor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Insert initial data for tenors
INSERT INTO tenors (duration_months, description) VALUES 
(1, '1 Months'),
(2, '2 Months'),
(3, '3 Months'),
(6, '6 Months'),
(9, '9 Months'),
(12, '1 Year'),
(18, '18 Months'),
(24, '2 Years'),
(36, '3 Years'),
(48, '4 Years'),
(60, '5 Years')
ON DUPLICATE KEY UPDATE description = VALUES(description);

-- Show table creation results
SELECT 'Database initialization completed successfully!' as message;
SHOW TABLES;