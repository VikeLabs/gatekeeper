package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

	hashedEmail, err := MakeIdentifier(guild, email)
	if err != nil {
		log.Println("failed making an identifier from the email:", err)
	}
	userID, ok := db.GetVerifiedEmail(hashedEmail)

	if ok && userID == user {
		err := addVerifiedRole(s, guild, user)
		return "Welcome back, you have been verified", err
	}

	// create token
	b := make([]byte, 4)
	rand.Read(b)
	token := hex.EncodeToString(b)

	db.SetEmailToken(hashedEmail, token)

	body := formatRegistrationEmail(token)

	// first, respond with some sort of "sending..." message
	// after that, send the email and edit the original message when we know if it succeeded
	// the bot needs to respond immediately with something, otherwise it'll time out
	defer func() {
		go func() {
			err := SendEmail(email, "Gatekeeper verification", body)
			if err != nil {
				log.Printf("error sending email to %v: %v\n", email, err)
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

	// put ban check after verification to prevent banned email enumeration
	if db.IsBanned(email) {
		return "You have been banned and are unable to verify.", nil
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
			oldUser, _ := s.User(oldUser)
			log.Println("error removing verified user:", oldUser.Username+"#"+oldUser.Discriminator)
		} else {
			msg = msg + "Your email was also used to verify <@" + oldUser.String() + ">. That account has been unverified.\n"
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

func Ban(s *state.State, user discord.UserID, guild discord.GuildID) (string, error) {
	email, ok := db.GetUserEmail(user)
	if !ok {
		return fmt.Sprintf("Error: user <@%v> not verified", user), nil
	}
	db.BanEmail(email)

	err := removeVerifiedRole(s, guild, user)
	if err != nil {
		return "", err
	}
	db.DeleteVerifiedEmail(email)

	return fmt.Sprintf("Success! User <@%v> was banned.", user), nil
}
