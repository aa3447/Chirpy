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
	"home/aa3447/workspace/github.com/aa3447/chirpy/internal/database"
	
	
	"github.com/joho/godotenv"
	
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
	serverMux.HandleFunc("POST /admin/reset", apiConfig.resetFileserverHitsHandler)
	serverMux.HandleFunc("POST /api/validate_chirp", validatePostHandler)

	serverStruct.ListenAndServe()
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func validatePostHandler(w http.ResponseWriter, r *http.Request) {
	type inputJSON struct {
		Body string `json:"body"`
	}
	type outputJSON struct {
		Error string `json:"error"`
		Cleaned_body string `json:"cleaned_body"`
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
		output.Error = "Chirp is too long"
		w.WriteHeader(400)
	} else {
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

		w.WriteHeader(http.StatusOK)
	}

	o, err := json.Marshal(output)
	if err != nil {
		log.Printf("Error encoding: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(o)
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

func (a *apiConfig) getFileserverHitsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	hits := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", a.fileserverHits.Load())
	w.Write([]byte(hits))
}

func (a *apiConfig) resetFileserverHitsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	a.fileserverHits.Store(0)
	w.Write([]byte("Hits reset to 0"))
}

func (a *apiConfig) incrementFileserverHits(handle http.Handler) http.Handler {

	handler := func(w http.ResponseWriter, r *http.Request) {
		a.fileserverHits.Add(1)
		handle.ServeHTTP(w, r)
	}

	handlerFunc := http.HandlerFunc(handler)

	return handlerFunc
}
