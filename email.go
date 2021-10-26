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

var botEmail = mustEnv("BOT_EMAIL")
var botPassword = mustEnv("BOT_PASSWORD")

var emailDialer = gomail.NewDialer(smtpHost, smtpPort, botEmail, botPassword)

func SendEmail(to, subject, body string) error {
	emailDialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	m := gomail.NewMessage()
	m.SetAddressHeader("From", botEmail, "Gatekeeper")
	m.SetAddressHeader("To", to, "")
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", emailifyNewlines(body))
	m.SetHeader("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:91.0) Gecko/20100101 Thunderbird/91.1.1")

	return emailDialer.DialAndSend(m)
}

func emailifyNewlines(in string) string {
	return strings.ReplaceAll(in, "\n", "\r\n")
}

func validateEmail(email string) error {
	address, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("email address format is invalid")
	}

	// is it the correct email domain?
	if !strings.HasSuffix(address.Address, "@"+EmailDomain) {
		return fmt.Errorf("email address must be a valid %s domain email address", EmailDomain)
	}

	// is it not an alias email address?
	if strings.Contains(address.Address, "+") {
		return fmt.Errorf("email address must not be an alias")
	}

	return nil
}
