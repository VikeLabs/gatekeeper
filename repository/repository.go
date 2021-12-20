package repository

import (
	"context"
	"errors"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	ErrGuildExists = errors.New("guild already exists")
)

type Storage interface {
	ListGuildIDs() ([]discord.GuildID, error)
}

type RepositoryStorage struct {
	pool *pgxpool.Pool
}

type Repository struct {
	storage *RepositoryStorage
}

func New(connString string) (*Repository, error) {
	// "postgres://username:password@localhost:5432/database_name"
	conf, err := pgxpool.ParseConfig(connString)

	if err != nil {
		return nil, err
	}
	// without pooling, this will result in a lot of blocking
	pool, err := pgxpool.ConnectConfig(context.Background(), conf)
	if err != nil {
		return nil, err
	}

	return &Repository{&RepositoryStorage{
		pool: pool,
	}}, err
}

func (r *Repository) Close() {
	r.storage.pool.Close()
}

// List of all the guilds in the database.
func (r *Repository) ListGuildIDs() ([]discord.GuildID, error) {
	const query = "SELECT id FROM guild"
	rows, err := r.storage.pool.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	ids := make([]discord.GuildID, rows.CommandTag().RowsAffected())

	for rows.Next() {
		var id discord.GuildID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Create a new guild in the database.
func (r *Repository) CreateGuild(guildID discord.GuildID) error {
	// where ON CONFLICT DO NOTHING is used to prevent duplicate guilds.
	const query = "INSERT INTO guild (id) VALUES ($1)"
	_, err := r.storage.pool.Exec(context.Background(), query, guildID)

	// does the guild already exist?
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgerrcode.UniqueViolation:
				return ErrGuildExists
			}
		}
	}

	return err
}

func (r *Repository) SetVerifiedEmail(guildID discord.GuildID, userID discord.UserID, email string) error {
	id := uuid.New()
	const query = "INSERT INTO email (id, guild_id, user_id, email) VALUES ($1, $2, $3, $4)"

	_, err := r.storage.pool.Exec(context.Background(), query, id, guildID, userID, email)
	return err
}

func (r *Repository) GetVerifiedEmail(guildID discord.GuildID, userID discord.UserID, email string) (*discord.UserID, error) {
	const query = "SELECT user_id FROM email WHERE guild_id = $1 AND user_id = $2 AND email = $3"
	var id discord.UserID
	if err := r.storage.pool.QueryRow(context.Background(), query, guildID, userID, email).Scan(&id); err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *Repository) SetGuildVerificationChannel(guildID discord.GuildID, channel discord.ChannelID) error {
	const query = "UPDATE guild SET verification_channel_id = $1 WHERE id = $2"
	_, err := r.storage.pool.Exec(context.Background(), query, channel, guildID)
	return err
}

func (r *Repository) GetGuildVerificationChannel(guildID discord.GuildID) (discord.ChannelID, error) {
	const query = "SELECT verification_channel_id FROM guild WHERE id = $1"
	var channel discord.ChannelID
	if err := r.storage.pool.QueryRow(context.Background(), query, guildID).Scan(&channel); err == pgx.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return channel, nil
}

func (r *Repository) InsertVerifiedEmail(guildID discord.GuildID, userID discord.UserID, email string) error {
	const query = "INSERT INTO email (id, guild_id, user_id, email) VALUES ($1, $2, $3, $4)"
	_, err := r.storage.pool.Exec(context.Background(), query, uuid.New(), guildID, userID, email)
	return err
}

// InsertEmailList inserts a list of emails into the database for the guild.
// If the last argument is specified as true, these emails will be denied verification regardless of domain.
func (r *Repository) InsertEmailList(guildID discord.GuildID, emails []string, blocked bool) error {
	batch := &pgx.Batch{}
	const query = "INSERT INTO email (id, guild_id, email, blocked) VALUES ($1, $2, $3, $4)"
	for _, email := range emails {
		id := uuid.New()
		batch.Queue(query, id, guildID, email, blocked)
	}
	br := r.storage.pool.SendBatch(context.Background(), batch)
	defer br.Close()
	_, err := br.Exec()
	if err != nil {
		return err
	}
	return nil
}

type DomainRole struct {
	Domain string
	RoleID discord.RoleID
}

func (r *Repository) InsertDomainRole(guildID discord.GuildID, domainRole *DomainRole) error {
	const query = "INSERT INTO domain (id, guild_id, domain, role_id) VALUES ($1, $2, $3, $4)"
	id := uuid.New()
	_, err := r.storage.pool.Exec(context.Background(), query, id, guildID, domainRole.Domain, domainRole.RoleID)
	return err
}

func (r *Repository) DeleteDomainRole(guildID discord.GuildID, domainRole *DomainRole) error {
	const query = "DELETE FROM domain WHERE guild_id = $1 AND domain = $2 AND role_id = $3"
	_, err := r.storage.pool.Exec(context.Background(), query, guildID, domainRole.Domain, domainRole.RoleID)
	if err == pgx.ErrNoRows {
		return nil
	}
	return err
}

// GetRolesByGuildAndDomain returns all roles that have been assigned to a domain.
func (r *Repository) GetRolesByGuildAndDomain(guildID discord.GuildID, domain string) (*[]discord.RoleID, error) {
	const query = "SELECT role_id FROM domain WHERE guild_id = $1 AND domain = $2"
	rows, err := r.storage.pool.Query(context.Background(), query, guildID, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]discord.RoleID, rows.CommandTag().RowsAffected())
	for rows.Next() {
		var role discord.RoleID
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return &roles, nil
}

// InsertToken inserts a token into the database.
func (r *Repository) InsertToken(guildID discord.GuildID, userID discord.UserID, email string, token string, domain string) error {
	const query = "INSERT INTO token (id, guild_id, user_id, email, token, domain) VALUES ($1, $2, $3, $4, $5, $6)"
	id := uuid.New()
	_, err := r.storage.pool.Exec(context.Background(), query, id, guildID, userID, email, token, domain)
	return err
}

// DleteTokens deletes all tokens for a user.
func (r *Repository) DeleteTokens(guildID discord.GuildID, userID discord.UserID, domain string) error {
	const query = "DELETE FROM token WHERE guild_id = $1 AND user_id = $2 AND domain = $3"
	_, err := r.storage.pool.Exec(context.Background(), query, guildID, userID, domain)
	return err
}

type TokenResponse struct {
	Email  string
	Domain string
	UserID discord.UserID
}

// GetToken returns the email, domain and user ID for the token.
func (r *Repository) GetToken(guildID discord.GuildID, token string) (*TokenResponse, error) {
	const query = `
		SELECT user_id, email, domain FROM token WHERE guild_id = $1 AND token = $2 AND "created_at" >= NOW() - INTERVAL '5 minutes' LIMIT 1
	`

	var res TokenResponse
	if err := r.storage.pool.QueryRow(context.Background(), query, guildID, token).Scan(&res.UserID, &res.Email, &res.Domain); err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &res, nil
}

// IsDomainValid returns true if the domain is valid for the guild.
func (r *Repository) IsDomainValid(guildID discord.GuildID, email string) (string, error) {
	// checks if the email matches the suffix of any of the domains for the guild.
	const query = "SELECT domain FROM domain WHERE guild_id = $1 AND $2 LIKE '%'||'@'||domain"
	var domain string
	if err := r.storage.pool.QueryRow(context.Background(), query, guildID, email).Scan(&domain); err == pgx.ErrNoRows {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return domain, nil
}

// IsEmailValid returns true if the email is valid for the guild.
// A valid email is not blocked, not used by another user unless its the same user.
func (r *Repository) IsEmailValid(guildID discord.GuildID, userID discord.UserID, email string) (bool, error) {
	// a email is consider valid if:
	// - it is not blocked
	// - it is not already used by another user UNLESS
	//  - it is already used by the user
	const query = `SELECT EXISTS(
		SELECT 1 FROM email WHERE 
			guild_id = $1 AND 
			(email = $3 AND blocked = TRUE) OR
			(email = $3 AND user_id != $2)
	)`
	var exists bool
	if err := r.storage.pool.QueryRow(context.Background(), query, guildID, userID, email).Scan(&exists); err != nil {
		return false, err
	}
	// if such email does not exist, it is valid
	return !exists, nil
}

// BanUser bans a user from the guild. This will block any subsequent verification attempts.
// Note that getting banned will ban all emails associated with the user.
func (r *Repository) BanUser(guildID discord.GuildID, userID discord.UserID) error {
	const query = "UPDATE email SET blocked = TRUE WHERE guild_id = $1 AND user_id = $2"
	_, err := r.storage.pool.Exec(context.Background(), query, guildID, userID)
	return err
}

// UnbanUser un-bans a user from the guild. This will allow any subsequent verification attempts.
// Note that they will need to re-verify to get their previous roles back.
func (r *Repository) UnbanUser(guildID discord.GuildID, userID discord.UserID) error {
	const query = "UPDATE email SET blocked = FALSE WHERE guild_id = $1 AND user_id = $2"
	_, err := r.storage.pool.Exec(context.Background(), query, guildID, userID)
	return err
}
