package internal

import (
	"os"
	"testing"
)

func TestLocalStackInitalization(t *testing.T) {
	os.Setenv("AWS_ACCESS_KEY_ID", "default")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "default")
	db := DataAdapter{}
	if db.New() == nil {
		t.Fail()
	}
}
