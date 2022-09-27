package main

import (
	"crypto/tls"
	"fmt"
	"net/mail"
	"strings"

	"gopkg.in/gomail.v2"
)

const smtpHost = "smtp.gmail.com"
const smtpPort = 587

var botEmail = mustEnv("GMAIL_EMAIL")
var botPassword = mustEnv("GMAIL_PASSWORD")

var emailDialer = gomail.NewDialer(smtpHost, smtpPort, botEmail, botPassword)

func SendEmail(to, subject, body string) error {
	emailDialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	m := gomail.NewMessage()
	m.SetAddressHeader("From", botEmail, "Gatekeeper")
	m.SetAddressHeader("To", to, "")
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", emailifyNewlines(body))

	return emailDialer.DialAndSend(m)
}

func emailifyNewlines(in string) string {
	return strings.ReplaceAll(in, "\n", "\r\n")
}

func validateEmail(domain, email string) error {
	address, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("email address format is invalid")
	}

	// is it the correct email domain?
	if !strings.HasSuffix(address.Address, "@"+domain) {
		return fmt.Errorf("email address must be a valid %s domain email address", domain)
	}

	// is it not an alias email address?
	if strings.Contains(address.Address, "+") {
		return fmt.Errorf("email address must not be an alias")
	}

	return nil
}

func extractDomain(email string) (string, error) {
	address, err := mail.ParseAddress(email)
	if err != nil {
		return "", fmt.Errorf("email address format is invalid")
	}

	parts := strings.Split(address.Address, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("email address format is invalid")
	}

	return parts[1], nil
}
