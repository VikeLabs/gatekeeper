-- NOTE: Make sure to read up on "Datatypes in SQLite" before you start writing
--       your own migrations. The SQLite documentation is available online at
--       https://www.sqlite.org/datatype3.html
CREATE TABLE verified (
	guild BIGINT NOT NULL,
	identifier BINARY(32) NOT NULL,
	verification_role BIGINT NOT NULL,
	user BIGINT NOT NULL,
	UNIQUE(identifier, user),
	PRIMARY KEY (guild, identifier)
);

CREATE INDEX verified_user_index on verified (guild, user);

CREATE TABLE token (
	guild BIGINT NOT NULL,
	token BINARY(8) NOT NULL,
	identifier BINARY(32) NOT NULL,
	verification_role BIGINT NOT NULL,
	created_at DATE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (guild, token)
);

CREATE INDEX tokens_created_at_index on token (created_at);

CREATE TABLE banned (
	guild BIGINT NOT NULL,
	identifier BINARY(32) NOT NULL,
	PRIMARY KEY (guild, identifier)
);

CREATE TABLE config (
	guild BIGINT NOT NULL,
	email_domain VARCHAR(255) NOT NULL,
	verification_role BIGINT NOT NULL,
	UNIQUE (guild, email_domain),
	PRIMARY KEY (guild, email_domain)
);