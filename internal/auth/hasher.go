package auth

import (
	"fmt"

	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error){
	if password == ""{
		return "", fmt.Errorf("please enter a valid password")
	}
	hash, err := argon2id.CreateHash(password,  argon2id.DefaultParams)

	return hash, err
}

func CheckPasswordHash(password, hash string) (bool, error){
	match, err := argon2id.ComparePasswordAndHash(password, hash)

	return match, err
}