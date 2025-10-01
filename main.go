package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"home/aa3447/workspace/github.com/aa3447/chirpy/internal/auth"
	"home/aa3447/workspace/github.com/aa3447/chirpy/internal/database"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	queries *database.Queries
	secret string
}
type validateChirpInputJSON struct {
	Body string `json:"body"`
}
type validateChirpOutputJSON struct {
	Cleaned_body string `json:"cleaned_body"`
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil{
		log.Printf("Error opening database: %s", err)
		os.Exit(1)
	}
	serverMux := http.NewServeMux()
	
	apiConfig := &apiConfig{}
	apiConfig.queries = database.New(db)
	apiConfig.secret = os.Getenv("JWTSECRET")

	serverStruct := &http.Server{
		Addr:    ":8080",
		Handler: serverMux,
	}

	serverMux.Handle("/app/", http.StripPrefix("/app", apiConfig.incrementFileserverHits(http.FileServer(http.Dir(".")))))
	serverMux.HandleFunc("GET /api/healthz", readinessHandler)
	serverMux.HandleFunc("GET /admin/metrics", apiConfig.getFileserverHitsHandler)
	serverMux.HandleFunc("GET /api/chirps/{chirpID}", apiConfig.getChirp)
	serverMux.HandleFunc("GET /api/chirps", apiConfig.getChirps)
	serverMux.HandleFunc("POST /admin/reset", apiConfig.resetFileserverHitsHandler)
	serverMux.HandleFunc("POST /api/chirps", apiConfig.validateChirp)
	serverMux.HandleFunc("POST /api/users", apiConfig.createUserHandle)
	serverMux.HandleFunc("POST /api/login", apiConfig.loginHandle)
	serverMux.HandleFunc("POST /api/refresh", apiConfig.refreshHandle)
	serverMux.HandleFunc("POST /api/revoke", apiConfig.revokeHandle)

	serverStruct.ListenAndServe()
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (a *apiConfig) validateChirp(w http.ResponseWriter, r *http.Request) {
	type chirp struct{
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	decoder := json.NewDecoder(r.Body)
	input := validateChirpInputJSON{}
	err := decoder.Decode(&input)
	errorHandler(w, err, "Error decoding", 500, true)

	token, err := auth.GetBearerToken(r.Header)
	errorHandler(w, err, "Error reading token", 401, false)
	
	userID , err := auth.ValidateJWT(token, a.secret)
	errorHandler(w, err, "Error validating token", 401, false)

	output := validateChirpOutputJSON{}
	if len(input.Body) > 140 {
		sendErrorJSON(w, fmt.Errorf("chirp is too long"), 400)
		return
	}
	
	valid, outputPointer := validateChirpInput(w, input.Body, &output)
	if !valid {
		errorHandler(w, fmt.Errorf("error validating chirp input"), "Error validating chirp input", 400, false)
		return
	}
	
	
	params := database.CreateChirpParams{
		Body: outputPointer.Cleaned_body,
		UserID: userID,
	}
	qChirp, err := a.queries.CreateChirp(r.Context(), params)
	errorHandler(w, err, "Error creating chirp", 500, true)

	a_chirp := chirp{
		ID: qChirp.ID,
  		CreatedAt: qChirp.CreatedAt,
  		UpdatedAt: qChirp.UpdatedAt,
  		Body: qChirp.Body,
  		UserID: qChirp.UserID,
	}

	sendJSON(w, a_chirp, 201)
}

func validateChirpInput(w http.ResponseWriter ,input string, output *validateChirpOutputJSON) (bool, *validateChirpOutputJSON) {
	if len(input) > 140 {
		sendErrorJSON(w, fmt.Errorf("chirp is too long"), 400)
		return false, &validateChirpOutputJSON{}
	}
	
	
	filterSlice := []string{"kerfuffle", "sharbert", "fornax"}
	tempString := input
	cutString := ""
	caseInsensitiveWord := ""
	for _, word := range filterSlice {
		currentIndex := strings.Index(strings.ToLower(tempString), word)
		for currentIndex > -1 {
			var byteSlice []byte
			caseInsensitiveWord = tempString[currentIndex:currentIndex + len(word)]
			
			if len(word) == len(tempString) {
				tempString = strings.Replace(tempString, caseInsensitiveWord, "****", 1)
				currentIndex = -2
			} else {
				if currentIndex == 0  {
					currentChar := tempString[len(word)]
					byteSlice = append(byteSlice, currentChar)
				} else if currentIndex+len(word) == len(tempString) {
					currentChar := tempString[currentIndex-1]
					byteSlice = append(byteSlice, currentChar)
				} else {
					backChar := tempString[currentIndex+len(word)]
					frontChar := tempString[currentIndex-1]
					byteSlice = append(byteSlice, backChar)
					byteSlice = append(byteSlice, frontChar)
				}
				if len(byteSlice) > 1 && filterMultiCharCheck(byteSlice) {
					tempString = strings.Replace(tempString, caseInsensitiveWord, "****", 1)
				} else if len(byteSlice) == 1 && filterCharCheck(byteSlice[0]) {
					tempString= strings.Replace(tempString, caseInsensitiveWord, "****", 1)
				} else {
					before, after, found := strings.Cut(tempString, caseInsensitiveWord)
					if found {
						cutString += before + word
						tempString = after
					}	
				}
				currentIndex = strings.Index(strings.ToLower(tempString), word)
			}	
		}
		
		if cutString == "" {
			output.Cleaned_body = tempString
		} else {
			output.Cleaned_body = cutString + tempString
		}
		tempString = output.Cleaned_body
		cutString = ""
	}

	return true, output
}

func filterCharCheck(currentChar byte) bool {
	currentCharRune := rune(currentChar)

	if unicode.IsLetter(currentCharRune) || currentCharRune == rune('!') || currentCharRune == rune('.') || currentCharRune == rune('?') {
		return false
	}
	return true
}

func filterMultiCharCheck(Chars []byte) bool {
	for _, currentChar := range Chars {
		if !filterCharCheck(currentChar) {
			return false
		}
	}
	return true
}

func errorHandler(w http.ResponseWriter, err error, errText string, headerNumber int, addLog bool) {
	if err != nil {
		if addLog {
			log.Printf("Error %s: %s", errText, err)
		}
		w.WriteHeader(headerNumber)
		w.Write([]byte(errText))
	}
}

func sendErrorJSON(w http.ResponseWriter, err error, errHeaderNumber int){
	type errorJSON struct {
		Error string `json:"error"`
	}
	errorOut := errorJSON{
		Error: err.Error(),
	}
	w.WriteHeader(errHeaderNumber)

	o, err := json.Marshal(errorOut)
	if err != nil {
		log.Printf("Error encoding: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(o)
}

func sendJSON(w http.ResponseWriter,  inputJSON any, headerNumber int){
	w.WriteHeader(headerNumber)

	o, err := json.Marshal(inputJSON)
	if err != nil {
		log.Printf("Error encoding: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(o)
}


func (a *apiConfig) getFileserverHitsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	hits := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", a.fileserverHits.Load())
	w.Write([]byte(hits))
}

func (a *apiConfig) getChirps(w http.ResponseWriter, r *http.Request) {
	type chirpsJSON struct{
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	var chirpsSlice []chirpsJSON
	
	chirps, err := a.queries.GetAllChirps(r.Context())
	errorHandler(w, err, "Error fetching chirps", 500, true)

	for _ , chirp := range chirps{
		a_chirp := chirpsJSON{
		ID: chirp.ID,
  		CreatedAt: chirp.CreatedAt,
  		UpdatedAt: chirp.UpdatedAt,
  		Body: chirp.Body,
  		UserID: chirp.UserID,
		}
		chirpsSlice = append(chirpsSlice, a_chirp)
	}


	sendJSON(w, chirpsSlice, 200)
}

func (a *apiConfig) getChirp(w http.ResponseWriter, r *http.Request) {
	type chirpJSON struct{
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	uID , err := uuid.Parse(r.PathValue("chirpID"))
	errorHandler(w, err, "Error reading chirp ID", 500, true)

	chirp, err := a.queries.GetChirp(r.Context(),uID)
	errorHandler(w, err, "Error fetching chirp", 500, true)

	a_chirp := chirpJSON{
		ID: chirp.ID,
  		CreatedAt: chirp.CreatedAt,
  		UpdatedAt: chirp.UpdatedAt,
  		Body: chirp.Body,
  		UserID: chirp.UserID,
	}

	sendJSON(w, a_chirp, 200)
}


func (a *apiConfig) resetFileserverHitsHandler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("PLATFORM") != "dev"{
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err := a.queries.ClearUsers(r.Context())
	errorHandler(w, err, "Error clearing users", 500, true)
	err = a.queries.ClearChirps(r.Context())
	errorHandler(w, err, "Error clearing chirps", 500, true)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	a.fileserverHits.Store(0)
	w.Write([]byte("Hits reset to 0\nUsers Cleared\nChirps cleared"))
}

func (a *apiConfig) incrementFileserverHits(handle http.Handler) http.Handler {

	handler := func(w http.ResponseWriter, r *http.Request) {
		a.fileserverHits.Add(1)
		handle.ServeHTTP(w, r)
	}

	handlerFunc := http.HandlerFunc(handler)

	return handlerFunc
}

func (a *apiConfig) createUserHandle(w http.ResponseWriter, r *http.Request){
	type inputJSON struct {
		Password string `json:"password"`
		Email string `json:"email"`
	}
	type outputJSON struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	input := inputJSON{}
	err := decoder.Decode(&input)
	errorHandler(w, err, "Error decoding", 500, true)

	hashed_password, err:= auth.HashPassword(input.Password)
	errorHandler(w, err, "Error hashing password", 500, true)

	params := database.CreateUserParams{
		Email: input.Email,
		HashedPassword: hashed_password,
	}

	user, err := a.queries.CreateUser(r.Context(), params)
	errorHandler(w, err, "Error creating user", 500, true)

	output := outputJSON{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
	}

	sendJSON(w,output,201)
}

func (a *apiConfig) loginHandle(w http.ResponseWriter, r *http.Request){
	type inputJSON struct {
		Password string `json:"password"`
		Email string `json:"email"`
	}
	type outputJSON struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string `json:"email"`
		Token string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	expiredJWT :=  "1h"
	expiredRefreshToken := "1440h"
	expireJWTdDuration , err := time.ParseDuration(expiredJWT)
	errorHandler(w, err, "Error parsing JWT time", 500, true)
	expireRefreshDuration , err := time.ParseDuration(expiredRefreshToken)
	errorHandler(w, err, "Error parsing refresh time", 500, true)

	decoder := json.NewDecoder(r.Body)
	input := inputJSON{}
	err = decoder.Decode(&input)
	errorHandler(w, err, "Error decoding", 500, true)

	user, err := a.queries.GetUserByEmail(r.Context(), input.Email)
	errorHandler(w, err, "Error fetching user", 401, true)

	same, err := auth.CheckPasswordHash(input.Password, user.HashedPassword)
	errorHandler(w, err, "Error checking password", 401, true)
	if !same {
		w.WriteHeader(401)
		w.Write([]byte("401 Unauthorized"))
		return
	}

	token, err := auth.MakeJWT(user.ID, a.secret , expireJWTdDuration)
	errorHandler(w, err, "Error making JWT token", 500, true)
	refreshToken, err := auth.MakeRefreshToken()
	errorHandler(w, err, "Error making refresh token", 500, true)

	params := database.CreateRefreshTokenParams{
		Token: refreshToken,
		UserID: user.ID,
		ExpiresAt: time.Now().Add(expireRefreshDuration) ,
		RevokedAt: sql.NullTime{Valid: false},
	}
	_, err = a.queries.CreateRefreshToken(r.Context(), params)
	errorHandler(w, err, "Error creating refresh token in database", 500, true)

	output := outputJSON{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
		Token: token,
		RefreshToken: refreshToken, 
	}
	
	sendJSON(w, output, 200)
}

func (a *apiConfig) refreshHandle(w http.ResponseWriter, r *http.Request){
	type outputJSON struct {
		Token string `json:"token"`
	}

	token, err := auth.GetBearerToken(r.Header)
	errorHandler(w, err, "Error reading token", 401, false)

	refreshToken , err := a.queries.GetRefreshToken(r.Context(), token)
	errorHandler(w, err, "Error fetching refresh token", 500, true)
	if !refreshToken.ExpiresAt.After(time.Now()) || refreshToken.RevokedAt.Valid{
		w.WriteHeader(401)
		w.Write([]byte("401 Unauthorized"))
		return
	}
	

	duration , err := time.ParseDuration("1h")
	errorHandler(w, err, "Error parsing time", 500, true)
	
	token, err = auth.MakeJWT(refreshToken.UserID, a.secret , duration)
	errorHandler(w, err, "Error making JWT token", 500, true)

	output := outputJSON{
		Token: token,
	}

	sendJSON(w,output,200)
}

func (a *apiConfig) revokeHandle(w http.ResponseWriter, r *http.Request){

	token, err := auth.GetBearerToken(r.Header)
	errorHandler(w, err, "Error reading token", 401, false)

	refreshToken , err := a.queries.GetRefreshToken(r.Context(), token)
	errorHandler(w, err, "Error fetching refresh token", 500, true)
	if !refreshToken.ExpiresAt.After(time.Now()) || refreshToken.RevokedAt.Valid{
		w.WriteHeader(401)
		w.Write([]byte("401 Unauthorized"))
		return
	}
	
	_, err = a.queries.UpdateRefreshToken(r.Context(), token)
	errorHandler(w, err, "Error updating refresh token", 500, true)
	
	w.WriteHeader(204)

}
