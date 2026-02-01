CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(255) PRIMARY KEY, 
    email VARCHAR(255) NOT NULL UNIQUE, 
    password_hash TEXT NOT NULL, 
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_password_hash_not_empty CHECK (length(password_hash) > 0),

    CONSTRAINT chk_email_lowercase CHECK (email = lower(email))
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

DROP TRIGGER IF EXISTS update_users_updated_at ON users;

CREATE TRIGGER update_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at_column();