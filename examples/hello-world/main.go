package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from MicroFoundry!")
	})
	fmt.Printf("Listening on :%s\n", port)
	http.ListenAndServe(":"+port, nil)
}
