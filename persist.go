package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
)

const persistenceFrequency = time.Second * 10

var dbFilename = mustEnv("DB_FILENAME")

type JSONDB struct {
	EmailTokens    map[string]string
	VerifiedEmails map[string]discord.UserID
}

func PersistenceRoutine(kill <-chan struct{}) {
	ticker := time.Tick(persistenceFrequency)

	for {
		select {
		case <-ticker:
			persistDB()
		case <-kill:
			persistDB()
			return
		}
	}
}

func persistDB() {
	db.EmailTokens.M.Lock()
	db.VerifiedEmails.M.Lock()
	jsonDB := JSONDB{
		EmailTokens:    db.EmailTokens.D,
		VerifiedEmails: db.VerifiedEmails.D,
	}
	jsonBytes, err := json.Marshal(jsonDB)
	if err != nil {
		log.Println("error marshalling db to json:", err)
	}
	db.EmailTokens.M.Unlock()
	db.VerifiedEmails.M.Unlock()

	err = os.WriteFile(dbFilename, jsonBytes, 0664)
	if err != nil {
		log.Println("error writing db to disk:", err)
	}
}

func loadDB() {
	jsonBytes, err := os.ReadFile(dbFilename)
	if err == os.ErrNotExist {
		return
	}
	if err != nil {
		log.Println("error reading db from disk:", err)
	}

	var jsonDB JSONDB
	err = json.Unmarshal(jsonBytes, &jsonDB)
	if err != nil {
		log.Println("error unmarshalling db from json:", err)
	}

	db.EmailTokens.M.Lock()
	db.EmailTokens.D = jsonDB.EmailTokens
	db.EmailTokens.M.Unlock()

	db.VerifiedEmails.M.Lock()
	db.VerifiedEmails.D = jsonDB.VerifiedEmails
	db.VerifiedEmails.M.Unlock()

	log.Printf("db successfully loaded from disk. %v members verified\n", len(db.VerifiedEmails.D))
}
