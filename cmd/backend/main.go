package main

import (
	"github.com/x0ddf/tiny-status-page/pkg/server"
	"log"
	"net/http"
	"os"
)

const DefaultPort = "8080"
const PortVar = "PORT"

func main() {
	var port string
	if port = os.Getenv(PortVar); port == "" {
		port = DefaultPort
	}

	// Initialize server
	srv, err := server.NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup HTTP handlers
	http.HandleFunc("/", srv.HandleIndex)
	http.HandleFunc("/ws", srv.HandleWebSocket)
	http.HandleFunc("/api/services", srv.HandleServices)
	http.HandleFunc("/api/contexts", srv.HandleContextList)
	http.HandleFunc("/api/contexts/switch", srv.HandleContextSwitch)

	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
