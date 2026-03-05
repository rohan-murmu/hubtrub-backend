package main

import (
	"log"
	"net/http"

	"github.com/scythrine05/hubtrub-server/internal/handler"
	"github.com/scythrine05/hubtrub-server/internal/middleware"
	"github.com/scythrine05/hubtrub-server/internal/repository"
	"github.com/scythrine05/hubtrub-server/internal/service"

	"github.com/gorilla/mux"
)

func main() {
	// Create a new Gorilla mux router
	router := mux.NewRouter()

	// Initialize repositories
	roomRepo := repository.NewFileRoomRepository("./data/room.jsonl")
	clientRepo := repository.NewFileClientRepository("./data/client.jsonl")

	// Initialize services
	roomService := service.NewRoomService(roomRepo)
	clientService := service.NewClientService(clientRepo)

	// Initialize handlers
	roomHandler := handler.NewRoomHandler(roomService)
	clientHandler := handler.NewClientHandler(clientService)

	// Initialize room manager
	roomManager := handler.NewRoomManager(100)

	// Register Client routes
	handler.RegisterClientRoutes(router, clientHandler)

	// Register Room routes
	handler.RegisterRoomRoutes(router, roomHandler)

	// WebSocket handler using Gorilla mux
	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handler.ServeWs(roomManager, roomService, clientService, w, r)
	})

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	log.Println("🚀 Hubtrub Server started on :9000")

	// Wrap the router with CORS middleware at the top level
	err := http.ListenAndServe(":9000", middleware.CORSMiddleware(router))
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
