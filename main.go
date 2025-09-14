package main

import (
	"net/http"
)

func main() {
	serverMux := http.NewServeMux()

	serverStruct := &http.Server{
		Addr: ":8080",
		Handler: serverMux,
	}

	serverMux.Handle("/", http.FileServer(http.Dir(".")))

	serverStruct.ListenAndServe()
}