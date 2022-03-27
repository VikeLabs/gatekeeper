package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

var VerificationRole = discord.RoleID(mustSnowflakeEnv("VERIFIED_ROLE_ID"))
var EmailDomain = mustEnv("EMAIL_DOMAIN")

func Register(s *state.State, editResponse func(string) error, user discord.UserID, guild discord.GuildID, email string) (string, error) {
	if err := validateEmail(email); err != nil {
		return err.Error(), nil
	}

	userID, ok := db.GetVerifiedEmail(email)

	if ok && userID == user {
		err := addVerifiedRole(s, guild, user)
		return "welcome back, you are verified", err
	}

	// create token
	b := make([]byte, 4)
	rand.Read(b)
	token := hex.EncodeToString(b)

	db.SetEmailToken(email, token)

	body := formatRegistrationEmail(token)

	// first, respond with some sort of "sending..." message
	// after that, send the email and edit the original message when we know if it succeeded
	// the bot needs to respond immediately with something, otherwise it'll time out
	defer func() {
		go func() {
			err := SendEmail(email, "Gatekeeper verification", body)
			if err != nil {
				log.Printf("Error sending email to %v: %v\n", email, err)
				err = editResponse("⚠️ Error sending email :(")
				if err != nil {
					log.Println("failed to send interaction callback:", err)
				}
				return
			} else {
				responseText := "✅ An email has been sent to " + email + "\nPlease use /verify <token> to verify your email address."
				err = editResponse(responseText)
				if err != nil {
					log.Println("failed to send interaction callback:", err)
				}
			}
		}()
	}()
	return "⌛ Sending email...", nil
}

func formatRegistrationEmail(token string) string {
	return ("Greetings from Gatekeeper!\n\n" +
		"Your verification token is: " + token)
}

func Verify(s *state.State, user discord.UserID, guild discord.GuildID, token string) (msg string, err error) {
	email, ok := db.GetEmailToken(token)
	if !ok {
		return "Sorry, verification failed.", nil
	}

	// we'll use to unverify the old user after verifying the new one
	oldUser, hasOldUser := db.GetVerifiedEmail(email)

	err = addVerifiedRole(s, guild, user)
	if err != nil {
		return "", err
	}

	msg += "Congrats! You've been verified!\n"

	db.DeleteEmailToken(token)
	db.SetVerifiedEmail(email, user)

	if hasOldUser {
		err := removeVerifiedRole(s, guild, oldUser)
		if err != nil {
			log.Println("")
		} else {
			msg = msg + "This email was in use by <@" + oldUser.String() + ">. They have been unverified.\n"
		}
	}

	return strings.TrimSpace(msg), nil
}

func addVerifiedRole(s *state.State, guild discord.GuildID, user discord.UserID) error {
	return s.AddRole(guild, user, VerificationRole, api.AddRoleData{AuditLogReason: api.AuditLogReason("Gatekeeper verification")})
}

func removeVerifiedRole(s *state.State, guild discord.GuildID, user discord.UserID) error {
	return s.RemoveRole(guild, user, VerificationRole, api.AuditLogReason("Gatekeeper verification"))
}
