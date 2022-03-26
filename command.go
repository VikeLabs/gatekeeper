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

func Register(s *state.State, editResponse func(string) error, user discord.UserID, guild discord.GuildID, email string) (string, error) {
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

	// first, respond with some sort of "waiting..."
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
