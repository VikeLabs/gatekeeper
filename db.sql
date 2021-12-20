-- Guild specific configuration
CREATE TABLE IF NOT EXISTS guild (
    id BIGINT NOT NULL PRIMARY KEY,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    is_encrypted BOOLEAN,
    verification_channel_id BIGINT
);

-- Verified Emails
CREATE TABLE IF NOT EXISTS email (
    id UUID PRIMARY KEY,
    guild_id BIGINT NOT NULL,
    user_id BIGINT,
    email VARCHAR(255) NOT NULL,
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Domain / Role Configuration
CREATE TABLE IF NOT EXISTS domain (
    id UUID PRIMARY KEY,
    guild_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    domain VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS token (
    id UUID PRIMARY KEY,
    guild_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    email VARCHAR(255) NOT NULL,
    domain VARCHAR(255) NOT NULL,
    token VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);