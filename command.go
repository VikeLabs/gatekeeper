package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

var VerificationRole = discord.RoleID(mustSnowflakeEnv("VERIFIED_ROLE_ID"))
var EmailDomain = mustEnv("EMAIL_DOMAIN")

func Register(s *state.State, user discord.UserID, guild discord.GuildID, email string) (string, error) {
	if err := validateEmail(email); err != nil {
		return err.Error(), nil
	}

	userID, ok := db.GetVerifiedEmail(email)

	if ok && userID == user {
		err := s.AddRole(guild, user, VerificationRole, api.AddRoleData{AuditLogReason: api.AuditLogReason("Gatekeeper verification")})
		return "welcome back, you are verified", err
	}

	if ok {
		return "email is currently claimed by a user", nil
	}

	// create token
	b := make([]byte, 4)
	rand.Read(b)
	token := hex.EncodeToString(b)

	db.SetEmailToken(email, token)

	body := formatRegistrationEmail(token)

	// put this in a goroutine because the interaction needs to respond quickly
	// if we want to display failure we should send an ephemeral message and edit it later
	go func() {
		err := SendEmail(email, "Gatekeeper verification", body)
		if err != nil {
			log.Printf("Error sending email to %v: %v\n", email, err)
		}
	}()
	return "A email has been sent to " + email + "\nPlease use /verify <token> to verify your email address.", nil
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

	oldUser, ok := db.GetVerifiedEmail(email)

	if ok {
		db.DeleteEmailToken(token)

		return "Sorry, this email is already in use by <@" + oldUser.String() + ">. Please contact a moderator to be verified manually .", nil
		// msg = msg + "This email was in use by <@" + oldUser.String() + ">. They will now be unverified."
		// // TODO ==================================================
		// oldMember :=
		// if (s.Member(guild, oldUser))
		// err = s.RemoveRole(guild, oldUser, VerificationRole, api.AuditLogReason("Gatekeeper verification"))
		// if err != nil {
		// 	return "", errors.Wrap(err, "cannot remove role from user")
		// }
	}

	err = s.AddRole(guild, user, VerificationRole, api.AddRoleData{AuditLogReason: api.AuditLogReason("Gatekeeper verification")})
	if err != nil {
		return "", err
	}

	db.DeleteEmailToken(token)
	db.SetVerifiedEmail(email, user)

	msg += "\nCongrats! You've been verified!"

	return msg, nil
}
