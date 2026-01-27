package main

import (
	"log"
	"net/http"

	"github.com/scythrine05/hubtrub-server/internal/handler"
	"github.com/scythrine05/hubtrub-server/internal/room"

	"github.com/gorilla/mux"
)

func main() {
	// Create a new Gorilla mux router
	router := mux.NewRouter()

	// In-memory storage for rooms (roomID -> Room)
	rooms := make(map[string]*room.Room)

	// WebSocket handler using Gorilla mux
	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handler.ServeWs(rooms, w, r)
	})

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	log.Println("🚀 Hubtrub Server started on :9000")
	log.Println("📡 WebSocket endpoint: ws://localhost:9000/ws?room_id=<room_id>")

	err := http.ListenAndServe(":9000", router)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
