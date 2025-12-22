-- +goose Up
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    username        VARCHAR(20) NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    email           VARCHAR(255), 

    nickname        VARCHAR(50),
    avatar_url      TEXT, 

    plan_type       VARCHAR(20) DEFAULT 'FREE' NOT NULL,
    plan_expires_at TIMESTAMPTZ, 
    
    created_at      TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at      TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    last_login_at   TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ, 

    CONSTRAINT username_check CHECK (username ~ '^[a-z0-9_]{4,20}$')
);

CREATE UNIQUE INDEX idx_users_username_active ON users (username) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX idx_users_email_active ON users (email) WHERE deleted_at IS NULL AND email IS NOT NULL;

-- +goose Down
-- NO-OP
