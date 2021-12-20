package main

import (
	"fmt"
	"log"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

const (
	ErrRegisteredOrBlockedEmail = "This email has either been registered or is blocked"
)

func Register(s *state.State, userID discord.UserID, guildID discord.GuildID, email string) (string, error) {
	// TODO: rate limit

	// basic checks like email format, not an alias etc.
	if err := validateEmailFormat(email); err != nil {
		return "Please insert a valid email", nil
	}

	// check if domain is valid
	domain, err := repo.IsDomainValid(guildID, email)
	if err != nil {
		return "", err
	}
	if domain == "" {
		// TODO: return a list of valid domains
		return "Please enter an email from an accepted domain name.", nil
	}

	msg := "A email has been sent to `" + email + "` if it was valid.\nPlease use `/verify <token>` to verify your email address."

	// check if the email can be used for registration (ie. not used or blocked)
	exists, err := repo.IsEmailValid(guildID, userID, email)
	if err != nil {
		return "", err
	}
	if exists {
		return msg, nil
	}

	token := makeToken()
	repo.InsertToken(guildID, userID, email, token, domain)

	// send email to user
	// in a go routine so we can return the message without waiting for the email to be sent
	go func() {
		SendEmail(email, "Gatekeeper verification", formatRegistrationEmail(token))
		if err != nil {
			log.Println("failed to send email:", err)
		}
	}()
	return msg, nil
}

func Verify(s *state.State, userID discord.UserID, guildID discord.GuildID, token string) (msg string, err error) {
	res, err := repo.GetToken(guildID, token)
	// issue with token retrieval
	if err != nil {
		return "", err
	}

	// token not found
	if res == nil {
		return "Token not found.", fmt.Errorf("invalid token")
	}

	// is token being used by another user?
	if res.UserID != userID {
		return "Please verify from the same account you initiated the registration", fmt.Errorf("user is attempting to verify from a different account")
	}

	// is this email being used by another user in the same guild?
	// on the small offchance that a user creates numerous tokens.
	exists, err := repo.IsEmailValid(guildID, userID, res.Email)
	if err != nil {
		return "", err
	}
	if exists {
		return ErrRegisteredOrBlockedEmail, fmt.Errorf("email is being used by another user")
	}

	// save the user details to the database as verified
	if err := repo.InsertVerifiedEmail(guildID, userID, res.Email); err != nil {
		return "", err
	}

	// add roles associated with the domain
	roles, err := repo.GetRolesByGuildAndDomain(guildID, res.Domain)
	if err != nil {
		return "", err
	}

	for _, role := range *roles {
		err = s.AddRole(guildID, userID, role, api.AddRoleData{AuditLogReason: api.AuditLogReason("Gatekeeper verification")})
		if err != nil {
			log.Println("error adding role:", err)
		}
	}

	// remove token
	err = repo.DeleteTokens(guildID, userID, res.Domain)
	if err != nil {
		log.Println("error deleting token:", err)
	}

	msg += "\nCongrats! You've been verified!"
	return msg, nil
}
