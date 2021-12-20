package main

import (
	"testing"
)

func TestEmailValidation(t *testing.T) {
	// invalid domain
	r := validateEmailFormat("bob@example.com")
	if r == nil {
		t.Error("email should be invalid")
	}
	// valid domain
	r = validateEmailFormat("bob@gatekeeper.com")
	if r != nil {
		t.Error("email should be valid")
	}

	// contains alias
	r = validateEmailFormat("bob+1@gatekeeper.com")
	if r == nil {
		t.Error("email should be invalid")
	}

	// invalid email
	r = validateEmailFormat("bob@")
	if r == nil {
		t.Error("email should be invalid")
	}
}
