package main

import (
	"sync"

	"github.com/diamondburned/arikawa/v3/discord"
)

type DB struct {
	EmailTokens struct {
		D map[string]string
		M sync.Mutex
	}
	VerifiedEmails struct {
		D map[string]discord.UserID
		M sync.Mutex
	}
}

var db = DB{
	EmailTokens: struct {
		D map[string]string
		M sync.Mutex
	}{D: map[string]string{}},
	VerifiedEmails: struct {
		D map[string]discord.UserID
		M sync.Mutex
	}{D: map[string]discord.UserID{}},
}
