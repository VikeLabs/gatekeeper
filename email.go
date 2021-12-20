package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"

	"golang.org/x/crypto/argon2"
	"gopkg.in/gomail.v2"
)

func SendEmail(to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetAddressHeader("From", botEmail, "Gatekeeper")
	m.SetAddressHeader("To", to, "")
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", emailifyNewlines(body))

	return emailDialer.DialAndSend(m)
}

func makeToken() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func emailifyNewlines(in string) string {
	return strings.ReplaceAll(in, "\n", "\r\n")
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmailFormat(email string) error {
	address, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("email address format is invalid")
	}

	// is it not an alias email address?
	if strings.Contains(address.Address, "+") {
		return fmt.Errorf("email address must not be an alias")
	}

	return nil
}

func hash(email string, guild uint64) []byte {
	guildBytes := new(bytes.Buffer)
	binary.Write(guildBytes, binary.BigEndian, guild)

	return argon2.IDKey(
		[]byte(email),
		guildBytes.Bytes(),
		1,       // time=1
		64*1024, // mem = 64MB
		1,       // 1 thread cause portability i guess
		32,      // 32 byte output key
	)
}

func formatRegistrationEmail(token string) string {
	return ("Greetings from Gatekeeper!\n\n" +
		"Your verification token is: " + token)
}
