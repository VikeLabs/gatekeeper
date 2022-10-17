package main

import (
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/base32"
	"errors"
	"fmt"
	"unsafe"

	"github.com/diamondburned/arikawa/v3/discord"
	_ "github.com/mattn/go-sqlite3"
)

// DONE move each function to SQLite
// DONE in functions with boolean return values, use ErrNoRows
// DONE check that the types are properly coerced
// TODO rename "email" to "identifier"
// TODO implement sql.Scan on Identifier (easier reading)

type Token [8]byte // TODO find a place to put this

func MakeToken() Token {
	var t Token
	rand.Read(t[:])
	return t
}

var enc = base32.HexEncoding.WithPadding(base32.NoPadding)

var _ fmt.Stringer = &Token{}

func (t *Token) String() string {
	return enc.EncodeToString(t[:])
}

var _ encoding.TextUnmarshaler = &Token{}

func (t *Token) UnmarshalText(text []byte) error {
	n, err := enc.Decode(t[:], text)
	if n != len(t) {
		return fmt.Errorf("token text should be %v bytes; got %v", len(t), n)
	}
	return err
}

type DBSnowflake discord.Snowflake

var _ sql.Scanner = (*DBSnowflake)(new(discord.Snowflake))

func (s *DBSnowflake) Scan(src any) error {
	intValue, ok := src.(int64)
	if !ok {
		return errors.New("expected int64 type")
	}
	*s = DBSnowflake(*(*uint64)(unsafe.Pointer(&intValue)))
	return nil
}

var _ driver.Valuer = DBSnowflake(0)

func (s DBSnowflake) Value() (driver.Value, error) {
	return *(*int64)(unsafe.Pointer(&s)), nil
}

var db DB

type DB struct {
	db *sql.DB
}

func InitDB() (DB, error) {
	// no need for _loc=auto, since we should use discord time things
	// https://discord.com/developers/docs/reference#message-formatting
	// TODO move db name to envvar
	dbConn, err := sql.Open("sqlite3", "db.sqlite")
	if err != nil {
		return DB{}, fmt.Errorf("error opening db: %w", err)
	}
	err = dbConn.Ping()
	if err != nil {
		return DB{}, fmt.Errorf("failed to establish connection to DB")
	}
	return DB{db: dbConn}, nil
}

// NOTE: positional arguments like $1, $2 ignore the number, so "$2, $1" behaves
// the same as "$1, $2". I think this is SQLite behaviour, but keep it for
// Postgres compatibility

func (d *DB) GetEmailToken(guild discord.GuildID, token Token) (Identifier, discord.RoleID, bool, error) {
	s := `
		SELECT identifier, verification_role FROM token 
		INNER JOIN config ON token.guild = config.guild AND token.email_domain = config.email_domain
		WHERE token = $1 AND token.guild = $2
	`
	row := d.db.QueryRow(s, token[:], DBSnowflake(guild))

	var idBuf []byte
	var snowflake DBSnowflake
	err := row.Scan(&idBuf, &snowflake)
	if errors.Is(err, sql.ErrNoRows) {
		return Identifier{}, discord.NullRoleID, false, nil
	}
	if err != nil {
		return Identifier{}, discord.NullRoleID, false, err
	}

	var id Identifier
	_, err = id.Write(idBuf)
	if err != nil {
		return Identifier{}, discord.NullRoleID, false, err
	}
	return id, discord.RoleID(snowflake), true, nil
}

func (d *DB) SetEmailToken(guild discord.GuildID, id Identifier, token Token, domain string) error {
	s := "INSERT INTO token (guild, token, identifier, email_domain) VALUES ($1,$2,$3,$4)"
	_, err := d.db.Exec(s, guild, token[:], id[:], domain)
	return err
}

func (d *DB) DeleteEmailToken(guild discord.GuildID, token Token) error {
	s := "DELETE FROM token WHERE token = $1 AND guild = $2"
	_, err := d.db.Exec(s, token[:], DBSnowflake(guild))
	return err
}

func (d *DB) GetVerifiedEmail(guild discord.GuildID, id Identifier) (discord.UserID, bool, error) {
	s := "SELECT user FROM verified WHERE identifier = $1 AND guild = $2"
	row := d.db.QueryRow(s, id[:], DBSnowflake(guild))
	var user DBSnowflake

	err := row.Scan(&user)
	if errors.Is(err, sql.ErrNoRows) {
		return discord.NullUserID, false, nil
	}
	if err != nil {
		return discord.NullUserID, false, err
	}

	return discord.UserID(user), true, nil
}

func (d *DB) SetVerifiedEmail(guild discord.GuildID, id Identifier, user discord.UserID, role discord.RoleID) error {
	s := "INSERT INTO verified (guild, identifier, user, verification_role) VALUES ($1,$2,$3,$4)"
	_, err := d.db.Exec(s, guild, id[:], DBSnowflake(user), DBSnowflake(role))
	return err
}

func (d *DB) DeleteVerifiedEmail(guild discord.GuildID, id Identifier) error {
	s := "DELETE FROM verified WHERE identifier = $1 AND guild = $2"
	_, err := d.db.Exec(s, id[:], DBSnowflake(guild))
	return err
}

func (d *DB) GetUserIdentifiers(guild discord.GuildID, user discord.UserID) ([]Identifier, error) {
	s := "SELECT identifier FROM verified WHERE user = $1 AND guild = $2"
	rows, err := d.db.Query(s, DBSnowflake(user), DBSnowflake(guild))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []Identifier
	for rows.Next() {
		var idBuf []byte
		err = rows.Scan(&idBuf)
		if err != nil {
			return nil, err
		}

		var id Identifier
		_, err = id.Write(idBuf)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (d *DB) BanEmail(guild discord.GuildID, id Identifier) error {
	s := "INSERT INTO banned (guild, identifier) VALUES ($1,$2)"
	_, err := d.db.Exec(s, DBSnowflake(guild), id[:])
	return err
}

func (d *DB) UnbanEmail(guild discord.GuildID, id Identifier) error {
	s := "DELETE FROM banned WHERE identifier = $1 AND guild = $2"
	_, err := d.db.Exec(s, id[:], DBSnowflake(guild))
	return err
}

func (d *DB) IsBanned(guild discord.GuildID, id Identifier) (bool, error) {
	s := "SELECT identifier FROM banned WHERE identifier = $1 AND guild = $2"
	row := d.db.QueryRow(s, id[:], DBSnowflake(guild))
	var tmp []byte
	err := row.Scan(&tmp)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func truncateDomain(domain string) string {
	domainRunes := []rune(domain)
	if len(domainRunes) > 255 {
		domainRunes = domainRunes[:255]
	}
	return string(domainRunes)
}

func (d *DB) UpdateConfig(guild discord.GuildID, domain string, role discord.RoleID) error {
	truncDomain := truncateDomain(domain)
	s := `
		INSERT INTO config (guild, email_domain, verification_role) VALUES ($1,$2,$3)
		ON CONFLICT (guild,email_domain) DO UPDATE 
		SET email_domain = $2,
			verification_role = $3
	`
	_, err := d.db.Exec(s, DBSnowflake(guild), truncDomain, DBSnowflake(role))
	return err
}

func (d *DB) DeleteConfig(guild discord.GuildID, domain string) error {
	s := "DELETE FROM config WHERE guild = $1 AND email_domain = $2"
	_, err := d.db.Exec(s, DBSnowflake(guild), domain)
	return err
}

func (d *DB) GetConfig(guild discord.GuildID, domain string) (discord.RoleID, bool, error) {
	s := "SELECT verification_role FROM config WHERE guild = $1 AND email_domain = $2"
	row := d.db.QueryRow(s, DBSnowflake(guild), domain)
	var role DBSnowflake
	err := row.Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return discord.NullRoleID, false, nil
	}
	if err != nil {
		return discord.NullRoleID, false, err
	}
	return discord.RoleID(role), true, nil
}

func (d *DB) EmailDomain(guild discord.GuildID) (string, bool, error) {
	s := "SELECT email_domain FROM config WHERE guild = $1"
	row := d.db.QueryRow(s, DBSnowflake(guild))
	var domain []byte
	err := row.Scan(&domain)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}
	return string(domain), true, nil
}

func (d *DB) VerificationRole(guild discord.GuildID, id Identifier) (discord.RoleID, bool, error) {
	s := "SELECT verification_role FROM verified WHERE guild = $1 AND identifier = $2"
	row := d.db.QueryRow(s, DBSnowflake(guild), id[:])
	var role DBSnowflake
	err := row.Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return discord.NullRoleID, false, nil
	} else if err != nil {
		return discord.NullRoleID, false, err
	}
	return discord.RoleID(role), true, nil
}

// Cleanup removes all tokens that are older than 5 minutes.
func (d *DB) CleanupTokens() error {
	s := "DELETE FROM tokens WHERE created_at < NOW() - INTERVAL '5 minutes'"
	_, err := d.db.Exec(s)
	return err
}
