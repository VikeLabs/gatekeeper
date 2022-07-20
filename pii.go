package main

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/diamondburned/arikawa/v3/discord"
	"golang.org/x/crypto/argon2"
)

const IdentifierLength = 32

type Identifier [IdentifierLength]byte

// email should already be validated
func MakeIdentifier(guild discord.GuildID, email string) (Identifier, error) {
	// these parameters were recommended by the docs for argon2.IDKey
	// https://pkg.go.dev/golang.org/x/crypto/argon2#IDKey
	const argon2Time = 1
	const argon2Mem = 64 * (1 << 10) // 64MB
	const argon2Threads = 1          // one thread cause portability i guess

	guildBytes := new(bytes.Buffer)
	binary.Write(guildBytes, binary.BigEndian, uint64(guild))

	tokenSlice := argon2.IDKey(
		[]byte(email),
		guildBytes.Bytes(),
		argon2Time,
		argon2Mem,
		argon2Threads,
		IdentifierLength,
	)

	if len(tokenSlice) != IdentifierLength {
		return Identifier{},
			fmt.Errorf("token should be %v bytes", IdentifierLength)
	}

	token := Identifier{}
	copy(token[:], tokenSlice)

	return token, nil
}

// for use as a value or map key in JSON
// https://pkg.go.dev/encoding/json#Marshal
// https://pkg.go.dev/encoding/json#Unmarshal
var _ encoding.TextMarshaler = Identifier{}    // must be value (for write)
var _ encoding.TextUnmarshaler = &Identifier{} // must be pointer (for read)

func (i *Identifier) UnmarshalText(b []byte) error {
	n, err := base64.StdEncoding.Decode(i[:], b)
	if err != nil {
		return err
	}
	if n != len(i) {
		return fmt.Errorf("bytes should have length %v", len(i))
	}
	return nil
}

func (i Identifier) MarshalText() ([]byte, error) {
	return []byte(base64.StdEncoding.EncodeToString(i[:])), nil
}

// for parsing from a []byte
var _ io.Writer = &Identifier{}

func (i *Identifier) Write(p []byte) (n int, err error) {
	if len(p) != len(i) {
		return 0, fmt.Errorf("bytes should have length %v", len(i))
	}
	n = copy(i[:], p)
	return
}
