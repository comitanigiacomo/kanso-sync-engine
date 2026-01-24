CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TABLE IF NOT EXISTS habits (
    id UUID PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,

    title VARCHAR(255) NOT NULL,
    description TEXT,
    color VARCHAR(7) DEFAULT '#9CA3AF',
    icon VARCHAR(50),
    sort_order INTEGER DEFAULT 0,

    type VARCHAR(50) NOT NULL CHECK (type IN ('boolean', 'timer', 'numeric')),
    frequency_type VARCHAR(50) NOT NULL CHECK (frequency_type IN ('daily', 'weekly', 'specific_days')),

    weekdays JSONB,
    reminder_time VARCHAR(10),

    interval INTEGER DEFAULT 1 CHECK (interval > 0),
    target_value INTEGER DEFAULT 1 CHECK (target_value > 0),
    unit VARCHAR(50),

    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE,
    archived_at TIMESTAMP WITH TIME ZONE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_habits_user_id ON habits(user_id);

CREATE TRIGGER update_habits_updated_at
BEFORE UPDATE ON habits
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at_column();