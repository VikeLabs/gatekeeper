package main

import (
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
)

const persistenceFrequency = time.Second * 10

var dbFilename = mustEnv("DB_FILENAME")

type JSONDB struct {
	EmailTokens    map[string]Identifier         `json:"email_tokens,omitempty"`
	VerifiedEmails map[Identifier]discord.UserID `json:"verified_emails,omitempty"`
}

func PersistenceRoutine(cleanedUp *sync.WaitGroup, kill <-chan struct{}) {
	ticker := time.NewTicker(persistenceFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			db.Persist()
		case <-kill:
			db.Persist()
			cleanedUp.Done()
			return
		}
	}
}
