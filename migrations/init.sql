CREATE TABLE verified (
	identifier BINARY(32) NOT NULL,
	user BIGINT NOT NULL,
	UNIQUE(identifier, user),
	PRIMARY KEY (identifier)
);
CREATE INDEX verified_user_index on verified (user);

CREATE TABLE token (
	token BINARY(8) NOT NULL,
	identifier BINARY(32) NOT NULL,
	created_at DATE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (token)
);
CREATE INDEX tokens_created_at_index on token (created_at);

CREATE TABLE banned (
	identifier BINARY(32) NOT NULL,
	PRIMARY KEY (identifier)
);

CREATE TABLE config (
	guild BIGINT NOT NULL,
	email_domain VARCHAR(255) NOT NULL,
	verification_role BIGINT NOT NULL,
	PRIMARY KEY (guild)
);