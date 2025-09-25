package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"unicode"
	"os"
	"database/sql"
	"time"
	"home/aa3447/workspace/github.com/aa3447/chirpy/internal/database"
	
	
	"github.com/joho/godotenv"
	"github.com/google/uuid"
	
	_ "github.com/lib/pq"

)

type apiConfig struct {
	fileserverHits atomic.Int32
	queries *database.Queries
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

	serverStruct.ListenAndServe()
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (a *apiConfig) validateChirp(w http.ResponseWriter, r *http.Request) {
	type inputJSON struct {
		Body string `json:"body"`
		User_id uuid.UUID `json:"user_id"`
	}
	type outputJSON struct {
		Cleaned_body string `json:"cleaned_body"`
	}
	type chirp struct{
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	filterSlice := []string{"kerfuffle", "sharbert", "fornax"}

	decoder := json.NewDecoder(r.Body)
	input := inputJSON{}
	err := decoder.Decode(&input)
	if err != nil {
		log.Printf("Error decoding: %s", err)
		w.WriteHeader(500)
		return
	}

	output := outputJSON{}
	if len(input.Body) > 140 {
		sendErrorJSON(w, fmt.Errorf("chirp is too long"), 400)
		return
	}
	
	tempString := input.Body
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
	
	
	params := database.CreateChirpParams{
		Body: output.Cleaned_body,
		UserID: input.User_id,
	}
	qChirp, err := a.queries.CreateChirp(r.Context(), params)
	if err != nil {
		log.Printf("Error creating chirp: %s", err)
		w.WriteHeader(500)
		return
	}

	a_chirp := chirp{
		ID: qChirp.ID,
  		CreatedAt: qChirp.CreatedAt,
  		UpdatedAt: qChirp.UpdatedAt,
  		Body: qChirp.Body,
  		UserID: qChirp.UserID,
	}

	sendJSON(w, a_chirp, 201)
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
	if err != nil{
		log.Printf("Error fetching chirp: %s", err)
		w.WriteHeader(500)
		return
	}

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
	if err != nil{
		log.Printf("Error reading chirp ID: %s", err)
		w.WriteHeader(500)
		return
	}

	chirp, err := a.queries.GetChirp(r.Context(),uID)
	if err != nil{
		log.Printf("Error fetching chirp: %s", err)
		w.WriteHeader(404)
		return
	}

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
	if err != nil{
		log.Printf("Error clearing users: %s", err)
		w.WriteHeader(500)
		return
	}
	err = a.queries.ClearChirps(r.Context())
	if err != nil{
		log.Printf("Error clearing chirps %s", err)
		w.WriteHeader(500)
		return
	}

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
	if err != nil {
		log.Printf("Error decoding: %s", err)
		w.WriteHeader(500)
		return
	}

	user, err := a.queries.CreateUser(r.Context(), input.Email)
	if err != nil{
		log.Printf("Error crating user: %s", err)
		w.WriteHeader(500)
		return
	}

	output := outputJSON{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
	}

	o, err := json.Marshal(output)
	if err != nil {
		log.Printf("Error encoding: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(o)
}
