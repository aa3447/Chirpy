package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error){
	claimsRegister := jwt.RegisteredClaims{
		Issuer: "chirpy",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject: userID.String(),
	}

	claim := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsRegister)
	signedClaim, err := claim.SignedString([]byte(tokenSecret))
	
	if err != nil {
		log.Printf("error: %s", err)
		return "", err
	}
	return signedClaim, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error){
	claims := jwt.RegisteredClaims{}
	keyFunc := func(token *jwt.Token) (any, error){
		return []byte(tokenSecret), nil
	}

	token, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc)
	if err != nil {
		log.Printf("error parsing token: %s", err)
		return uuid.Max, err
	}

	expired, err := token.Claims.GetExpirationTime()
	if err != nil {
		log.Printf("error retrieving id: %s", err)
		return uuid.Max, err
	}
	if !expired.Time.After(time.Now()){
		log.Printf("error token expired: %s", err)
		return uuid.Max, fmt.Errorf("token expired")
	}

	idString ,err:= token.Claims.GetSubject()
	if err != nil {
		log.Printf("error retrieving id: %s", err)
		return uuid.Max, err
	}

	userID , err := uuid.Parse(idString)
	if err != nil {
		log.Printf("error parsing id: %s", err)
		return uuid.Max, err
	}

	return userID, nil
}

func GetBearerToken(headers http.Header) (string, error){
	fullToken := headers.Get("Authorization")
	if fullToken == ""{
		return "", fmt.Errorf("error reading header")
	}

	token := strings.TrimSpace(strings.Split(fullToken, "Bearer")[1])
	
	return token, nil
}

func MakeRefreshToken() (string, error){
	key := make([]byte, 32)
	rand.Read(key)

	return hex.EncodeToString(key), nil
}
