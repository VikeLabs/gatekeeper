package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/diamondburned/arikawa/v3/discord"
)

type DB struct {
	EmailTokens struct {
		D map[string]Identifier
		M sync.Mutex
	}
	VerifiedEmails struct {
		D map[Identifier]discord.UserID
		M sync.Mutex
	}
	BannedEmails struct {
		D map[Identifier]bool
		M sync.Mutex
	}
}

func (d *DB) GetEmailToken(token string) (Identifier, bool) {
	d.EmailTokens.M.Lock()
	defer d.EmailTokens.M.Unlock()

	email, ok := d.EmailTokens.D[token]
	return email, ok
}

func (d *DB) SetEmailToken(email Identifier, token string) {
	d.EmailTokens.M.Lock()
	defer d.EmailTokens.M.Unlock()

	d.EmailTokens.D[token] = email
}

func (d *DB) DeleteEmailToken(token string) {
	d.EmailTokens.M.Lock()
	defer d.EmailTokens.M.Unlock()
	delete(d.EmailTokens.D, token)
}

func (d *DB) GetVerifiedEmail(email Identifier) (discord.UserID, bool) {
	d.VerifiedEmails.M.Lock()
	defer d.VerifiedEmails.M.Unlock()

	id, ok := d.VerifiedEmails.D[email]
	return id, ok
}

func (d *DB) SetVerifiedEmail(email Identifier, id discord.UserID) {
	d.VerifiedEmails.M.Lock()
	defer d.VerifiedEmails.M.Unlock()

	d.VerifiedEmails.D[email] = id
}

func (d *DB) DeleteVerifiedEmail(email Identifier) {
	d.VerifiedEmails.M.Lock()
	defer d.VerifiedEmails.M.Unlock()

	delete(d.VerifiedEmails.D, email)
}

// O(n) time
func (d *DB) GetUserEmail(uid discord.UserID) (Identifier, bool) {
	d.VerifiedEmails.M.Lock()
	defer d.VerifiedEmails.M.Unlock()

	for k, v := range d.VerifiedEmails.D {
		if v == uid {
			return k, true
		}
	}
	return Identifier{}, false
}

func (d *DB) BanEmail(email Identifier) {
	d.BannedEmails.M.Lock()
	defer d.BannedEmails.M.Unlock()

	d.BannedEmails.D[email] = true
}

func (d *DB) UnbanEmail(email Identifier) {
	d.BannedEmails.M.Lock()
	defer d.BannedEmails.M.Unlock()

	delete(d.BannedEmails.D, email)
}

func (d *DB) IsBanned(email Identifier) bool {
	d.BannedEmails.M.Lock()
	defer d.BannedEmails.M.Unlock()

	_, banned := d.BannedEmails.D[email]
	return banned
}

func (d *DB) Persist() {
	d.EmailTokens.M.Lock()
	d.VerifiedEmails.M.Lock()
	d.BannedEmails.M.Lock()
	jsonDB := JSONDB{
		EmailTokens:    d.EmailTokens.D,
		VerifiedEmails: d.VerifiedEmails.D,
		BannedEmails:   d.BannedEmails.D,
	}
	jsonBytes, err := json.Marshal(jsonDB)
	if err != nil {
		log.Println("error marshalling db to json:", err)
	}
	d.EmailTokens.M.Unlock()
	d.VerifiedEmails.M.Unlock()
	d.BannedEmails.M.Unlock()

	err = os.WriteFile(dbFilename, jsonBytes, 0664)
	if err != nil {
		log.Println("error writing db to disk:", err)
	}
}

func (d *DB) Load() {
	jsonBytes, err := os.ReadFile(dbFilename)
	if err == os.ErrNotExist {
		return
	}
	if err != nil {
		log.Println("error reading db from disk:", err)
		return
	}

	var jsonDB JSONDB
	err = json.Unmarshal(jsonBytes, &jsonDB)
	if err != nil {
		log.Println("error unmarshalling db from json:", err)
	}

	d.EmailTokens.M.Lock()
	if jsonDB.EmailTokens != nil {
		d.EmailTokens.D = jsonDB.EmailTokens
	} else {
		d.EmailTokens.D = map[string]Identifier{}
	}
	d.EmailTokens.M.Unlock()

	d.VerifiedEmails.M.Lock()
	if jsonDB.VerifiedEmails != nil {
		d.VerifiedEmails.D = jsonDB.VerifiedEmails
	} else {
		d.VerifiedEmails.D = map[Identifier]discord.UserID{}
	}
	d.VerifiedEmails.M.Unlock()

	d.VerifiedEmails.M.Lock()
	if jsonDB.VerifiedEmails != nil {
		d.VerifiedEmails.D = jsonDB.VerifiedEmails
	} else {
		d.VerifiedEmails.D = map[Identifier]discord.UserID{}
	}
	d.VerifiedEmails.M.Unlock()

	log.Printf("db successfully loaded from disk. %v members verified\n", len(d.VerifiedEmails.D))
}

var db = DB{
	EmailTokens: struct {
		D map[string]Identifier
		M sync.Mutex
	}{D: map[string]Identifier{}},
	VerifiedEmails: struct {
		D map[Identifier]discord.UserID
		M sync.Mutex
	}{D: map[Identifier]discord.UserID{}},
	BannedEmails: struct {
		D map[Identifier]bool
		M sync.Mutex
	}{D: map[Identifier]bool{}},
}
