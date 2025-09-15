package main

import (
	"net/http"
	"sync/atomic"
	"fmt"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}


func main() {
	serverMux := http.NewServeMux()
	apiConfig := &apiConfig{}

	serverStruct := &http.Server{
		Addr: ":8080",
		Handler: serverMux,
	}

	serverMux.Handle("/app/", http.StripPrefix("/app" , apiConfig.incrementFileserverHits(http.FileServer(http.Dir(".")))))
	serverMux.HandleFunc("/healthz", readinessHandler)
	serverMux.HandleFunc("/metrics", apiConfig.getFileserverHitsHandler)
	serverMux.HandleFunc("/reset", apiConfig.resetFileserverHitsHandler)

	serverStruct.ListenAndServe()
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (a *apiConfig) getFileserverHitsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	hits := fmt.Sprintf("Hits: %d", a.fileserverHits.Load())
	w.Write([]byte(hits))
}

func (a *apiConfig) resetFileserverHitsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	a.fileserverHits.Store(0)
	w.Write([]byte("Hits reset to 0"))
}

func (a *apiConfig) incrementFileserverHits(handle http.Handler) http.Handler {

	handler := func (w http.ResponseWriter, r *http.Request) {
			a.fileserverHits.Add(1)	
			handle.ServeHTTP(w, r)
		}

	handlerFunc := http.HandlerFunc(handler)

	return handlerFunc
}
