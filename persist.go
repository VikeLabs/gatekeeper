package main

import (
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
)

const persistenceFrequency = time.Second * 10

var dbFilename = mustEnv("DB_FILENAME")

type JSONDB struct {
	EmailTokens    map[string]string         `json:"email_tokens,omitempty"`
	VerifiedEmails map[string]discord.UserID `json:"verified_emails,omitempty"`
}

func PersistenceRoutine(kill <-chan struct{}) {
	ticker := time.NewTicker(persistenceFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			db.Persist()
		case <-kill:
			db.Persist()
			return
		}
	}
}
