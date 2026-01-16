-- Migration 027: User Timezone Settings (Story 7.6)
-- Adds timezone column to users table and creates timezone presets table

-- Add timezone column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'Asia/Kolkata';

-- Create timezone presets table
CREATE TABLE IF NOT EXISTS timezone_presets (
    id SERIAL PRIMARY KEY,
    display_name VARCHAR(100) NOT NULL,
    tz_identifier VARCHAR(50) NOT NULL UNIQUE,
    gmt_offset VARCHAR(10) NOT NULL,
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Insert default presets (ON CONFLICT on tz_identifier to allow re-running)
INSERT INTO timezone_presets (display_name, tz_identifier, gmt_offset, is_default) VALUES
    ('India Standard Time (IST)', 'Asia/Kolkata', '+05:30', true),
    ('Indochina Time (ICT)', 'Asia/Phnom_Penh', '+07:00', false),
    ('Coordinated Universal Time (UTC)', 'UTC', '+00:00', false)
ON CONFLICT (tz_identifier) DO NOTHING;

-- Create index for faster timezone lookups
CREATE INDEX IF NOT EXISTS idx_users_timezone ON users(timezone);
