package main

import (
	"crypto/rand"
	"database/sql"
	"encoding"
	"encoding/base32"
	"errors"
	"fmt"

	"github.com/diamondburned/arikawa/v3/discord"
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

func (d *DB) GetEmailToken(guild discord.GuildID, token Token) (Identifier, bool, error) {
	s := "SELECT identifier FROM token WHERE token = $1 AND guild = $2"
	row := d.db.QueryRow(s, token[:], guild)

	var idBuf []byte
	err := row.Scan(&idBuf)
	if errors.Is(err, sql.ErrNoRows) {
		return Identifier{}, false, nil
	}
	if err != nil {
		return Identifier{}, false, err
	}

	var id Identifier
	_, err = id.Write(idBuf)
	if err != nil {
		return Identifier{}, false, err
	}

	return id, true, nil
}

func (d *DB) SetEmailToken(guild discord.GuildID, email Identifier, token Token) error {
	s := "INSERT INTO token (guild, token, identifier) VALUES ($1,$2,$3)"
	_, err := d.db.Exec(s, guild, token[:], email[:])
	return err
}

func (d *DB) DeleteEmailToken(guild discord.GuildID, token Token) error {
	s := "DELETE FROM token WHERE token = $1 AND guild = $2"
	_, err := d.db.Exec(s, token[:], guild)
	return err
}

func (d *DB) GetVerifiedEmail(guild discord.GuildID, id Identifier) (discord.UserID, bool, error) {
	s := "SELECT user FROM verified WHERE identifier = $1 AND guild = $2"
	row := d.db.QueryRow(s, id[:], guild)

	var user discord.UserID
	err := row.Scan(&user)
	if errors.Is(err, sql.ErrNoRows) {
		return discord.NullUserID, false, nil
	}
	if err != nil {
		return discord.NullUserID, false, err
	}

	return user, true, nil
}

func (d *DB) SetVerifiedEmail(guild discord.GuildID, id Identifier, user discord.UserID) error {
	s := "INSERT INTO verified (guild, identifier, user) VALUES ($1,$2,$3)"
	_, err := d.db.Exec(s, guild, id[:], user)
	return err
}

func (d *DB) DeleteVerifiedEmail(guild discord.GuildID, id Identifier) error {
	s := "DELETE FROM verified WHERE identifier = $1 AND guild = $2"
	_, err := d.db.Exec(s, id[:], guild)
	return err
}

func (d *DB) DeleteVerifiedUser(guild discord.GuildID, user discord.UserID) error {
	s := "DELETE FROM verified WHERE user = $1 AND guild = $2"
	_, err := d.db.Exec(s, user, guild)
	return err
}

func (d *DB) GetUserEmail(guild discord.GuildID, user discord.UserID) (Identifier, bool, error) {
	s := "SELECT identifier FROM verified WHERE user = $1 AND guild = $2"
	row := d.db.QueryRow(s, user, guild)

	var idBuf []byte
	err := row.Scan(&idBuf)
	if errors.Is(err, sql.ErrNoRows) {
		return Identifier{}, false, nil
	}
	if err != nil {
		return Identifier{}, false, err
	}

	var id Identifier
	_, err = id.Write(idBuf)
	if err != nil {
		return Identifier{}, false, err
	}

	return id, true, err
}

func (d *DB) BanEmail(guild discord.GuildID, id Identifier) error {
	s := "INSERT INTO banned (guild, identifier) VALUES ($1,$2)"
	_, err := d.db.Exec(s, guild, id[:])
	return err
}

func (d *DB) UnbanEmail(guild discord.GuildID, id Identifier) error {
	s := "DELETE FROM banned WHERE identifier = $1 AND guild = $2"
	_, err := d.db.Exec(s, id[:], guild)
	return err
}

func (d *DB) IsBanned(guild discord.GuildID, id Identifier) (bool, error) {
	s := "SELECT identifier FROM banned WHERE identifier = $1 AND guild = $2"
	row := d.db.QueryRow(s, id[:], guild)
	var tmp []byte
	err := row.Scan(&tmp)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func truncateDomain(domain string) []byte {
	domainBytes := []byte(domain)
	if len(domainBytes) > 255 {
		domainBytes = domainBytes[:255]
	}
	return domainBytes
}

func (d *DB) MakeConfig(guild discord.GuildID, domain string, role discord.RoleID) error {
	domainBytes := truncateDomain(domain)
	s := "INSERT INTO config (guild, email_domain, verification_role) VALUES ($1,$2,$3)"
	_, err := d.db.Exec(s, guild, domainBytes, role)
	return err
}

func (d *DB) UpdateConfig(guild discord.GuildID, domain string, role discord.RoleID) error {
	domainBytes := truncateDomain(domain)
	s := "UPDATE config SET email_domain = $1, verification_role = $2 WHERE guild = $3"
	_, err := d.db.Exec(s, domainBytes, role, guild)
	return err
}

func (d *DB) HasConfig(guild discord.GuildID) (bool, error) {
	s := "SELECT guild FROM config WHERE guild = $1"
	row := d.db.QueryRow(s, guild)
	var tmp discord.GuildID
	err := row.Scan(&tmp)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (d *DB) EmailDomain(guild discord.GuildID) (string, bool, error) {
	s := "SELECT email_domain FROM config WHERE guild = $1"
	row := d.db.QueryRow(s, guild)
	var domain []byte
	err := row.Scan(&domain)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}
	return string(domain), true, nil
}

func (d *DB) VerificationRole(guild discord.GuildID) (discord.RoleID, bool, error) {
	s := "SELECT verification_role FROM config WHERE guild = $1"
	row := d.db.QueryRow(s, guild)
	var role discord.RoleID
	err := row.Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return discord.NullRoleID, false, nil
	} else if err != nil {
		return discord.NullRoleID, false, err
	}
	return role, true, nil
}

// Cleanup removes all tokens that are older than 5 minutes.
func (d *DB) CleanupTokens() error {
	s := "DELETE FROM tokens WHERE created_at < NOW() - INTERVAL '5 minutes'"
	_, err := d.db.Exec(s)
	return err
}
