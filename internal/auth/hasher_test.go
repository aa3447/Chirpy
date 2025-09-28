package auth

import (
	"testing"
)

func TestHash(t *testing.T) {
	testPassword := "abcd"
	hashedPassword, err := HashPassword(testPassword)

	if err != nil {
		t.Errorf("Hash unsuccessful: %s", err)
		return
	}
	if testPassword == hashedPassword {
		t.Error("Hash should not match normal password")
		return
	}
}

func TestHashEmptyString(t *testing.T) {
	testPassword := ""
	_, err := HashPassword(testPassword)

	if err == nil {
		t.Errorf("Empty string should be disallowed: %s", err)
		return
	}
}

func TestHashCheck(t *testing.T) {
	testPassword := "abcd"
	hashedPassword, _ := HashPassword(testPassword)

	match, err := CheckPasswordHash(testPassword, hashedPassword)
	if err != nil {
		t.Errorf("Hash unsuccessful: %s", err)
		return
	}
	if !match {
		t.Error("Hash match unsuccessful")
		return
	}
}
