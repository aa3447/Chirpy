package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakingJWT(t *testing.T) {
	userID := uuid.New()
	duration, err := time.ParseDuration("1h")
	if err != nil {
		t.Errorf("time parse error: %s", err)
		return
	}
	
	jwt, err := MakeJWT(userID, "asdfgsdafdga", duration)

	if  err != nil{
		t.Errorf("error making jwt: %s", err)
		return
	}
	if jwt == ""{
		t.Errorf("error making jwt")
		return
	}
}

func TestJWTValidation(t *testing.T) {
	userID := uuid.New()
	duration, err := time.ParseDuration("1h")
	secret := "asdfgsdafdga"
	if err != nil {
		t.Errorf("time parse error: %s", err)
		return
	}
	
	jwt, _ := MakeJWT(userID, secret, duration)

	parsedUserID, err := ValidateJWT(jwt,secret)

	if  err != nil{
		t.Errorf("error checking jwt: %s", err)
		return
	}
	if userID != parsedUserID{
		t.Errorf("userid mismatch")
		return
	}
}

func TestJWTExpire(t *testing.T) {
	userID := uuid.New()
	duration, err := time.ParseDuration("1s")
	secret := "asdfgsdafdga"
	if err != nil {
		t.Errorf("time parse error: %s", err)
		return
	}
	
	jwt, _ := MakeJWT(userID, secret, duration)
	time.Sleep(duration)


	_, err = ValidateJWT(jwt,secret)

	if err == nil{
		t.Errorf("token should be expired: %s", err)
		return
	}
}
