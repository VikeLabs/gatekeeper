package main

import (
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
		// validateEmail gives helpful errors on invalid emails
		return err.Error(), nil
	}

	hashedEmail, err := MakeIdentifier(guild, email)
	if err != nil {
		return "", fmt.Errorf("failed making an identifier from the email: %w", err)
	}
	userID, ok, err := db.GetVerifiedEmail(hashedEmail)
	if err != nil {
		return "", fmt.Errorf("error getting user ID from DB during registration: %w", err)
	}

	if ok && userID == user {
		err := addVerifiedRole(s, guild, user)
		return "Welcome back, you have been verified", err
	}

	// create token
	token := MakeToken()

	err = db.SetEmailToken(hashedEmail, token)
	if err != nil {
		return "", fmt.Errorf("error setting token in DB: %v", err)
	}

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
				const format = "✅ An email has been sent to %v\nPlease use /verify <token> to verify your email address."
				responseText := fmt.Sprintf(format, email)
				err = editResponse(responseText)
				if err != nil {
					log.Println("failed to send interaction callback:", err)
				}
			}
		}()
	}()
	return "⌛ Sending email...", nil
}

func formatRegistrationEmail(token Token) string {
	return fmt.Sprintf(
		"Greetings from Gatekeeper!\n\n"+
			"Your verification token is: %v", token.String())
}

func Verify(s *state.State, user discord.UserID, guild discord.GuildID, tokenString string) (string, error) {
	var token Token
	err := (&token).UnmarshalText([]byte(tokenString))
	if err != nil {
		return "", fmt.Errorf("error unmarshalling token: %w", err)
	}

	msg := &strings.Builder{}

	email, ok, err := db.GetEmailToken(token)
	if err != nil {
		return "", fmt.Errorf("error getting token from db: %w", err)
	}
	if !ok {
		return "Your token is incorrect.", nil
	}

	// put ban check after verification to prevent banned email enumeration
	banned, err := db.IsBanned(email)
	if err != nil {
		return "", fmt.Errorf("error checking if user is banned: %w", err)
	}
	if banned {
		return "You have been banned and are unable to verify.", nil
	}

	// we'll use to unverify the old user after verifying the new one
	oldUser, hasOldUser, err := db.GetVerifiedEmail(email)
	if err != nil {
		return "", fmt.Errorf("error getting user from db: %w", err)
	}

	// must remove old user before adding new one for UNIQUE constraint
	if hasOldUser {
		err := removeVerifiedRole(s, guild, oldUser)
		if err != nil {
			oldUser, _ := s.User(oldUser)
			return "", fmt.Errorf("error removing verified user: %v#%v", oldUser.Username, oldUser.Discriminator)
		} else {
			fmt.Fprintf(msg, "Your email was also used to verify <@%v>. That account has been unverified.\n", oldUser)
		}
		err = db.DeleteVerifiedUser(oldUser)
		if err != nil {
			return "", fmt.Errorf("error removing verified user from db: %w", err)
		}

	}

	err = addVerifiedRole(s, guild, user)
	if err != nil {
		return "", fmt.Errorf("couldn't verify user: %w", err)
	}

	msg.WriteString("Congrats! You've been verified!\n")

	err = db.DeleteEmailToken(token)
	if err != nil {
		return "", fmt.Errorf("error deleting token in DB: %w", err)
	}
	err = db.SetVerifiedEmail(email, user)
	if err != nil {
		return "", fmt.Errorf("error verifying user in DB: %w", err)
	}

	return strings.TrimSpace(msg.String()), nil
}

func addVerifiedRole(s *state.State, guild discord.GuildID, user discord.UserID) error {
	return s.AddRole(guild, user, VerificationRole, api.AddRoleData{AuditLogReason: api.AuditLogReason("Gatekeeper verification")})
}

func removeVerifiedRole(s *state.State, guild discord.GuildID, user discord.UserID) error {
	return s.RemoveRole(guild, user, VerificationRole, api.AuditLogReason("Gatekeeper verification"))
}

func Ban(s *state.State, user discord.UserID, guild discord.GuildID) (string, error) {
	email, ok, err := db.GetUserEmail(user)
	if err != nil {
		return "", fmt.Errorf("error getting identifier from DB: %w", err)
	}
	if !ok {
		return fmt.Sprintf("Error: user <@%v> not verified", user), nil
	}
	err = db.BanEmail(email)
	if err != nil {
		return "", fmt.Errorf("error banning id in DB: %w", err)
	}

	err = removeVerifiedRole(s, guild, user)
	if err != nil {
		return "", fmt.Errorf("couldn't unverify user: %w", err)
	}
	err = db.DeleteVerifiedEmail(email)
	if err != nil {
		return "", fmt.Errorf("error unverifying user in DB: %w", err)
	}

	return fmt.Sprintf("Success! User <@%v> was banned.", user), nil
}
