-- Expand invoice profile text fields to support real-world Chinese invoice data.
ALTER TABLE invoice_profiles
    ALTER COLUMN title TYPE VARCHAR(200),
    ALTER COLUMN address TYPE VARCHAR(500),
    ALTER COLUMN bank_name TYPE VARCHAR(200);
