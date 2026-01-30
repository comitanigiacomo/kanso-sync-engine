CREATE TABLE IF NOT EXISTS habit_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    habit_id UUID NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL, 
    
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