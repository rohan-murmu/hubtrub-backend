package main

import (
	"log"
	"net/http"

	"github.com/scythrine05/hubtrub-server/internal/handler"
	"github.com/scythrine05/hubtrub-server/internal/repository"
	"github.com/scythrine05/hubtrub-server/internal/room"
	"github.com/scythrine05/hubtrub-server/internal/service"

	"github.com/gorilla/mux"
)

func main() {
	// Create a new Gorilla mux router
	router := mux.NewRouter()

	// In-memory storage for rooms (roomID -> Room)
	rooms := make(map[string]*room.Room)

	// Initialize repositories
	roomRepo := repository.NewFileRoomRepository("./data/room.jsonl")
	clientRepo := repository.NewFileClientRepository("./data/client.jsonl")

	// Initialize services
	roomService := service.NewRoomService(roomRepo)
	clientService := service.NewClientService(clientRepo)

	// Initialize handlers
	roomHandler := handler.NewRoomHandler(roomService)
	clientHandler := handler.NewClientHandler(clientService)

	// Register Client routes
	handler.RegisterClientRoutes(router, clientHandler)

	// Register Room routes
	handler.RegisterRoomRoutes(router, roomHandler)

	// WebSocket handler using Gorilla mux
	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handler.ServeWs(rooms, roomService, w, r)
	})

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	log.Println("🚀 Hubtrub Server started on :9000")

	err := http.ListenAndServe(":9000", router)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
