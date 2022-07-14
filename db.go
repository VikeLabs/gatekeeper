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

func (d *DB) GetEmailToken(token Token) (Identifier, bool, error) {
	s := "SELECT identifier FROM token WHERE token = $1"
	row := d.db.QueryRow(s, token[:])

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

func (d *DB) SetEmailToken(email Identifier, token Token) error {
	s := "INSERT INTO token (token, identifier) VALUES ($1,$2)"
	_, err := d.db.Exec(s, token[:], email[:])
	return err
}

func (d *DB) DeleteEmailToken(token Token) error {
	s := "DELETE FROM token WHERE token = $1"
	_, err := d.db.Exec(s, token[:])
	return err
}

func (d *DB) GetVerifiedEmail(id Identifier) (discord.UserID, bool, error) {
	s := "SELECT user FROM verified WHERE identifier = $1"
	row := d.db.QueryRow(s, id[:])

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

func (d *DB) SetVerifiedEmail(id Identifier, user discord.UserID) error {
	s := "INSERT INTO verified (identifier, user) VALUES ($1,$2)"
	_, err := d.db.Exec(s, id[:], user)
	return err
}

func (d *DB) DeleteVerifiedEmail(id Identifier) error {
	s := "DELETE FROM verified WHERE identifier = $1"
	_, err := d.db.Exec(s, id[:])
	return err
}

func (d *DB) DeleteVerifiedUser(user discord.UserID) error {
	s := "DELETE FROM verified WHERE user = $1"
	_, err := d.db.Exec(s, user)
	return err
}

func (d *DB) GetUserEmail(user discord.UserID) (Identifier, bool, error) {
	row := d.db.QueryRow("SELECT identifier FROM verified WHERE user = $1", user)

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

func (d *DB) BanEmail(id Identifier) error {
	_, err := d.db.Exec("INSERT INTO banned (identifier) VALUES ($1)", id[:])
	return err
}

func (d *DB) UnbanEmail(id Identifier) error {
	_, err := d.db.Exec("DELETE FROM banned WHERE identifier = $1", id[:])
	return err
}

func (d *DB) IsBanned(id Identifier) (bool, error) {
	row := d.db.QueryRow("SELECT identifier FROM banned WHERE identifier = $1", id[:])
	var tmp []byte
	err := row.Scan(&tmp)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
