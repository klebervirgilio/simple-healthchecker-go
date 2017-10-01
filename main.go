package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/klebervirgilio/simple-healthchecker-go/healthcheck"
)

func main() {
	port := os.Getenv("WEB_SERVER_PORT")
	fmt.Printf("Listening on port %s\n", port)
	http.HandleFunc("/healthcheck/", healthcheck.Handler)
	http.HandleFunc("/parallel-healthcheck/", healthcheck.ParallelHandler)

	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Could not initialize server: %s", err)
	}
}
