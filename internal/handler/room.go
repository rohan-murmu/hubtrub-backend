package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/scythrine05/hubtrub-server/internal/middleware"
	"github.com/scythrine05/hubtrub-server/internal/model"
	"github.com/scythrine05/hubtrub-server/internal/service"
)

// RoomHandler handles room HTTP requests
type RoomHandler struct {
	service *service.RoomService
}

// NewRoomHandler creates a new room handler
func NewRoomHandler(service *service.RoomService) *RoomHandler {
	return &RoomHandler{service: service}
}

// CreateRoom handles POST /room - creates a room with authenticated client as admin
func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client from context
	clientUserName, ok := middleware.GetClientUserNameFromContext(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var room model.Room
	if err := json.NewDecoder(r.Body).Decode(&room); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Auto-generate roomID and set creation timestamp
	room.RoomID = uuid.New().String()
	room.RoomCreatedAt = time.Now()
	room.RoomAdmin = clientUserName

	if err := h.service.CreateRoom(&room); err != nil {
		http.Error(w, "Failed to create room", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(room)
}

// GetAllRooms handles GET /room
func (h *RoomHandler) GetAllRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.service.GetAllRooms()
	if err != nil {
		http.Error(w, "Failed to fetch rooms", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)
}

// GetRoomByID handles GET /room/:room_id
func (h *RoomHandler) GetRoomByID(w http.ResponseWriter, r *http.Request) {
	roomID := mux.Vars(r)["room_id"]

	room, err := h.service.GetRoomByID(roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(room)
}

// UpdateRoom handles PUT /room/:room_id - only admin can update
func (h *RoomHandler) UpdateRoom(w http.ResponseWriter, r *http.Request) {
	roomID := mux.Vars(r)["room_id"]

	// Get authenticated client from context
	clientUserName, ok := middleware.GetClientUserNameFromContext(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify user is the room admin
	existingRoom, err := h.service.GetRoomByID(roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	if existingRoom.RoomAdmin != clientUserName {
		http.Error(w, "Forbidden: Only room admin can update", http.StatusForbidden)
		return
	}

	var room model.Room
	if err := json.NewDecoder(r.Body).Decode(&room); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure the room ID and admin match
	room.RoomID = roomID
	room.RoomAdmin = clientUserName

	if err := h.service.UpdateRoom(roomID, &room); err != nil {
		http.Error(w, "Failed to update room", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(room)
}

// DeleteRoom handles DELETE /room/:room_id - only admin can delete
func (h *RoomHandler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	roomID := mux.Vars(r)["room_id"]

	// Get authenticated client from context
	clientUserName, ok := middleware.GetClientUserNameFromContext(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify user is the room admin
	room, err := h.service.GetRoomByID(roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	if room.RoomAdmin != clientUserName {
		http.Error(w, "Forbidden: Only room admin can delete", http.StatusForbidden)
		return
	}

	if err := h.service.DeleteRoom(roomID); err != nil {
		http.Error(w, "Failed to delete room", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// RegisterRoomRoutes registers all room routes
func RegisterRoomRoutes(router *mux.Router, handler *RoomHandler) {
	// Protected routes (require auth)
	protectedRouter := router.NewRoute().Subrouter()
	protectedRouter.Use(middleware.AuthMiddleware)
	protectedRouter.HandleFunc("/room", handler.CreateRoom).Methods("POST")
	protectedRouter.HandleFunc("/room/{room_id}", handler.UpdateRoom).Methods("PUT")
	protectedRouter.HandleFunc("/room/{room_id}", handler.DeleteRoom).Methods("DELETE")

	// Public routes
	router.HandleFunc("/room", handler.GetAllRooms).Methods("GET")
	router.HandleFunc("/room/{room_id}", handler.GetRoomByID).Methods("GET")
}
