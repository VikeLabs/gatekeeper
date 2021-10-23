package main

import (
	"crypto/tls"
	"strings"

	"gopkg.in/gomail.v2"
)

var smtpHost = "smtp.gmail.com"
var smtpPort = 587

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
