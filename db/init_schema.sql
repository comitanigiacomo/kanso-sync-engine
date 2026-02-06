CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ language 'plpgsql';

-- USERS table

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

-- HABITS table

CREATE TABLE IF NOT EXISTS habits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- FIX: Aggiunta FK
    
    title VARCHAR(255) NOT NULL,
    description TEXT,
    color VARCHAR(7) DEFAULT '#9CA3AF',
    icon VARCHAR(50),
    sort_order INTEGER DEFAULT 0,
    
    type VARCHAR(50) NOT NULL CHECK (type IN ('boolean', 'timer', 'numeric')),
    frequency_type VARCHAR(50) NOT NULL CHECK (frequency_type IN ('daily', 'weekly', 'specific_days', 'interval')),
    weekdays JSONB,
    reminder_time VARCHAR(10),
    
    interval INTEGER DEFAULT 1 CHECK (interval > 0),
    target_value INTEGER DEFAULT 1 CHECK (target_value > 0),
    unit VARCHAR(50),

    current_streak INTEGER DEFAULT 0 CHECK (current_streak >= 0),
    longest_streak INTEGER DEFAULT 0 CHECK (longest_streak >= 0),
    
    start_date TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    end_date TIMESTAMP WITH TIME ZONE,
    archived_at TIMESTAMP WITH TIME ZONE,
    
    version INTEGER DEFAULT 1 NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_habits_user_id ON habits(user_id);
CREATE INDEX IF NOT EXISTS idx_habits_updated_at ON habits(updated_at);

DROP TRIGGER IF EXISTS update_habits_updated_at ON habits;
CREATE TRIGGER update_habits_updated_at
BEFORE UPDATE ON habits
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at_column();

-- Tabella HABIT ENTRIES TABLE

CREATE TABLE IF NOT EXISTS habit_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    habit_id UUID NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    completion_date TIMESTAMP WITH TIME ZONE NOT NULL,
    value INTEGER DEFAULT 1,
    notes TEXT,
    
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT habit_entries_value_check CHECK (value >= 0)
);

CREATE INDEX IF NOT EXISTS idx_habit_entries_user_updated ON habit_entries(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_habit_entries_habit_date ON habit_entries(habit_id, completion_date);

DROP TRIGGER IF EXISTS update_habit_entries_updated_at ON habit_entries;
CREATE TRIGGER update_habit_entries_updated_at
BEFORE UPDATE ON habit_entries
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at_column();