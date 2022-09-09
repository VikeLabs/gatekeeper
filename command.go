package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

func Register(s *state.State, editResponse func(string) error, user discord.UserID, guild discord.GuildID, email string) (string, error) {
	domain, ok, err := db.EmailDomain(guild)
	if err != nil {
		return "", fmt.Errorf("failed to get email domain from DB: %w", err)
	} else if !ok {
		return "You need to configure the verifiable emails, ask your admins to set it up.", err
	}
	if err := validateEmail(string(domain), email); err != nil {
		// validateEmail gives helpful errors on invalid emails
		return err.Error(), nil
	}

	hashedEmail, err := MakeIdentifier(guild, email)
	if err != nil {
		return "", fmt.Errorf("failed making an identifier from the email: %w", err)
	}

	// skip registration if account should already be verified
	userID, ok, err := db.GetVerifiedEmail(guild, hashedEmail)
	if err != nil {
		return "", fmt.Errorf("error getting user ID from DB during registration: %w", err)
	}
	if ok && userID == user {
		ok, err := addVerifiedRole(s, guild, user)
		if err != nil {
			return "", fmt.Errorf("couldn't verify user: %w", err)
		} else if !ok {
			return "You need to configure the verified role first, ask your admins to set it up.", err
		}
		return "Welcome back, you have been verified", err
	}

	// create random token
	token := MakeToken()
	err = db.SetEmailToken(guild, hashedEmail, token)
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

	// message may have multiple lines so we use a string builder
	msg := &strings.Builder{}

	email, ok, err := db.GetEmailToken(guild, token)
	if err != nil {
		return "", fmt.Errorf("error getting token from db: %w", err)
	}
	if !ok {
		return "Your token is incorrect.", nil
	}

	// put ban check after verification to prevent banned email enumeration
	banned, err := db.IsBanned(guild, email)
	if err != nil {
		return "", fmt.Errorf("error checking if user is banned: %w", err)
	}
	if banned {
		return "You have been banned and are unable to verify.", nil
	}

	// we'll use to unverify the old user after verifying the new one
	oldUser, hasOldUser, err := db.GetVerifiedEmail(guild, email)
	if err != nil {
		return "", fmt.Errorf("error getting user from db: %w", err)
	}

	// must remove old user before adding new one for UNIQUE constraint
	if hasOldUser {
		ok, err := removeVerifiedRole(s, guild, oldUser)
		if err != nil {
			oldUser, _ := s.User(oldUser)
			return "", fmt.Errorf("error removing verified user: %v#%v", oldUser.Username, oldUser.Discriminator)
		} else if !ok {
			return "You need to configure the verified role first, ask your admins to set it up.", err
		} else {
			fmt.Fprintf(msg, "Your email was also used to verify <@%v>. That account has been unverified.\n", oldUser)
		}
		err = db.DeleteVerifiedUser(guild, oldUser)
		if err != nil {
			return "", fmt.Errorf("error removing verified user from db: %w", err)
		}

	}

	ok, err = addVerifiedRole(s, guild, user)
	if err != nil {
		return "", fmt.Errorf("couldn't verify user: %w", err)
	} else if !ok {
		return "You need to configure the verified role first, ask your admins to set it up.", err
	}

	err = db.DeleteEmailToken(guild, token)
	if err != nil {
		return "", fmt.Errorf("error deleting token in DB: %w", err)
	}
	err = db.SetVerifiedEmail(guild, email, user)
	if err != nil {
		return "", fmt.Errorf("error verifying user in DB: %w", err)
	}

	msg.WriteString("Congrats! You've been verified!\n")

	return strings.TrimSpace(msg.String()), nil
}

func addVerifiedRole(s *state.State, guild discord.GuildID, user discord.UserID) (bool, error) {
	role, ok, err := db.VerificationRole(guild)
	if err != nil {
		return false, err
	} else if !ok {
		return false, nil
	}
	return true, s.AddRole(guild, user, role, api.AddRoleData{AuditLogReason: api.AuditLogReason("Gatekeeper verification")})
}

func removeVerifiedRole(s *state.State, guild discord.GuildID, user discord.UserID) (bool, error) {
	role, ok, err := db.VerificationRole(guild)
	if err != nil {
		return false, err
	} else if !ok {
		return false, nil
	}
	return true, s.RemoveRole(guild, user, role, api.AuditLogReason("Gatekeeper verification"))
}

func Ban(s *state.State, user discord.UserID, guild discord.GuildID) (string, error) {
	email, ok, err := db.GetUserEmail(guild, user)
	if err != nil {
		return "", fmt.Errorf("error getting identifier from DB: %w", err)
	}
	if !ok {
		return fmt.Sprintf("Error: user <@%v> not verified", user), nil
	}
	err = db.BanEmail(guild, email)
	if err != nil {
		return "", fmt.Errorf("error banning id in DB: %w", err)
	}

	ok, err = removeVerifiedRole(s, guild, user)
	if err != nil {
		return "", fmt.Errorf("couldn't unverify user: %w", err)
	} else if !ok {
		return "You need to configure the verified role first, ask your admins to set it up.", err
	}
	err = db.DeleteVerifiedEmail(guild, email)
	if err != nil {
		return "", fmt.Errorf("error unverifying user in DB: %w", err)
	}

	return fmt.Sprintf("Success! User <@%v> was banned.", user), nil
}

func Config(s *state.State, guild discord.GuildID, domain string, role discord.RoleID) (string, error) {
	if has, err := db.HasConfig(guild); err != nil {
		return "", err
	} else if has {
		err := db.UpdateConfig(guild, domain, role)
		if err != nil {
			return "", err
		}
	} else {
		err := db.MakeConfig(guild, domain, role)
		if err != nil {
			return "", err
		}
	}
	return "Successfully updated config!", nil
}
