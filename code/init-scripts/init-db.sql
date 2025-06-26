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
(6, '6 Months'),
(12, '1 Year'),
(18, '18 Months'),
(24, '2 Years'),
(36, '3 Years'),
(48, '4 Years'),
(60, '5 Years')
ON DUPLICATE KEY UPDATE description = VALUES(description);

-- Insert sample customer data for testing (optional)
INSERT INTO customers (
    nik, 
    full_name, 
    legal_name, 
    birth_place, 
    birth_date, 
    salary, 
    ktp_photo_url, 
    selfie_photo_url, 
    verification_status
) VALUES 
(
    '1234567890123456', 
    'John Doe', 
    'John Doe', 
    'Jakarta', 
    '1990-01-15', 
    5000000.00, 
    'https://example.com/ktp/john_doe.jpg', 
    'https://example.com/selfie/john_doe.jpg', 
    'VERIFIED'
),
(
    '2345678901234567', 
    'Jane Smith', 
    'Jane Smith', 
    'Bandung', 
    '1985-05-20', 
    7500000.00, 
    'https://example.com/ktp/jane_smith.jpg', 
    'https://example.com/selfie/jane_smith.jpg', 
    'PENDING'
),
(
    '3456789012345678', 
    'Bob Wilson', 
    'Robert Wilson', 
    'Surabaya', 
    '1992-12-10', 
    6000000.00, 
    'https://example.com/ktp/bob_wilson.jpg', 
    'https://example.com/selfie/bob_wilson.jpg', 
    'VERIFIED'
)
ON DUPLICATE KEY UPDATE 
    full_name = VALUES(full_name),
    legal_name = VALUES(legal_name);

-- Insert sample customer limits for testing
INSERT INTO customer_limits (customer_id, tenor_id, limit_amount)
SELECT 
    c.id,
    t.id,
    CASE 
        WHEN t.duration_months = 6 THEN c.salary * 2
        WHEN t.duration_months = 12 THEN c.salary * 3
        WHEN t.duration_months = 18 THEN c.salary * 4
        WHEN t.duration_months = 24 THEN c.salary * 5
        WHEN t.duration_months = 36 THEN c.salary * 6
        WHEN t.duration_months = 48 THEN c.salary * 7
        WHEN t.duration_months = 60 THEN c.salary * 8
        ELSE c.salary * 2
    END as limit_amount
FROM customers c
CROSS JOIN tenors t
WHERE c.verification_status = 'VERIFIED'
ON DUPLICATE KEY UPDATE 
    limit_amount = VALUES(limit_amount);

-- Insert sample transaction for testing
INSERT INTO transactions (
    contract_number,
    customer_id,
    tenor_id,
    asset_name,
    otr_amount,
    admin_fee,
    total_interest,
    total_installment_amount,
    status
)
SELECT 
    CONCAT('LOAN-', YEAR(CURDATE()), '-', LPAD(c.id, 6, '0'), '-001'),
    c.id,
    t.id,
    'Honda Beat 2023',
    15000000.00,
    250000.00,
    1800000.00,
    17050000.00,
    'ACTIVE'
FROM customers c
JOIN tenors t ON t.duration_months = 12
WHERE c.nik = '1234567890123456'
LIMIT 1
ON DUPLICATE KEY UPDATE 
    asset_name = VALUES(asset_name);

-- Create additional useful views
CREATE OR REPLACE VIEW customer_summary AS
SELECT 
    c.id,
    c.nik,
    c.full_name,
    c.verification_status,
    c.salary,
    COUNT(t.id) as total_transactions,
    COALESCE(SUM(CASE WHEN t.status = 'ACTIVE' THEN t.total_installment_amount ELSE 0 END), 0) as active_loan_amount,
    COALESCE(MAX(cl.limit_amount), 0) as max_limit
FROM customers c
LEFT JOIN transactions t ON c.id = t.customer_id
LEFT JOIN customer_limits cl ON c.id = cl.customer_id
GROUP BY c.id, c.nik, c.full_name, c.verification_status, c.salary;

-- Create view for transaction summary
CREATE OR REPLACE VIEW transaction_summary AS
SELECT 
    t.id,
    t.contract_number,
    c.full_name as customer_name,
    c.nik as customer_nik,
    t.asset_name,
    tn.duration_months,
    tn.description as tenor_description,
    t.otr_amount,
    t.total_installment_amount,
    t.status,
    t.transaction_date
FROM transactions t
JOIN customers c ON t.customer_id = c.id
JOIN tenors tn ON t.tenor_id = tn.id;

-- Show table creation results
SELECT 'Database initialization completed successfully!' as message;
SHOW TABLES;