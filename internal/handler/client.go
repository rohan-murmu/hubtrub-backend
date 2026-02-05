package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/scythrine05/hubtrub-server/internal/model"
	"github.com/scythrine05/hubtrub-server/internal/service"
	"github.com/scythrine05/hubtrub-server/internal/util"
)

// ClientRequest represents the request body for creating a client
type ClientRequest struct {
	ClientUserName string `json:"clientUserName"`
	ClientAvatar   string `json:"clientAvatar"`
}

// ClientResponse wraps client data with auth token
type ClientResponse struct {
	Client *model.Client `json:"client"`
	Token  string        `json:"token"`
}

// ClientHandler handles client HTTP requests
type ClientHandler struct {
	service *service.ClientService
}

// NewClientHandler creates a new client handler
func NewClientHandler(service *service.ClientService) *ClientHandler {
	return &ClientHandler{service: service}
}

// CreateClient handles POST /client
func (h *ClientHandler) CreateClient(w http.ResponseWriter, r *http.Request) {
	var clientReq ClientRequest
	if err := json.NewDecoder(r.Body).Decode(&clientReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if client with this username already exists
	existingClient, err := h.service.GetClientByUserName(clientReq.ClientUserName)
	if err == nil && existingClient != nil {
		// Username exists - update the existing client with a new ClientID
		newID := uuid.New().String()
		existingClient.ClientID = newID
		existingClient.ClientAvatar = clientReq.ClientAvatar
		if err := h.service.UpdateClient(existingClient.ClientID, existingClient); err != nil {
			http.Error(w, "Failed to update client", http.StatusInternalServerError)
			return
		}

		// Generate JWT token for the updated client
		token, err := util.GenerateToken(existingClient.ClientID, existingClient.ClientUserName)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ClientResponse{
			Client: existingClient,
			Token:  token,
		})
		return
	}

	// Create new client with auto-generated ID
	client := model.Client{
		ClientID:       uuid.New().String(),
		ClientUserName: clientReq.ClientUserName,
		ClientAvatar:   clientReq.ClientAvatar,
	}

	if err := h.service.CreateClient(&client); err != nil {
		http.Error(w, "Failed to create client", http.StatusInternalServerError)
		return
	}

	// Generate JWT token for the client
	token, err := util.GenerateToken(client.ClientID, client.ClientUserName)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ClientResponse{
		Client: &client,
		Token:  token,
	})
}

// GetAllClients handles GET /client
func (h *ClientHandler) GetAllClients(w http.ResponseWriter, r *http.Request) {
	clients, err := h.service.GetAllClients()
	if err != nil {
		http.Error(w, "Failed to fetch clients", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

// GetClientByID handles GET /client/:client_id
func (h *ClientHandler) GetClientByID(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["client_id"]

	client, err := h.service.GetClientByID(clientID)
	if err != nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(client)
}

// UpdateClient handles PUT /client/:client_id
func (h *ClientHandler) UpdateClient(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["client_id"]

	var client model.Client
	if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure the client ID matches
	client.ClientID = clientID

	if err := h.service.UpdateClient(clientID, &client); err != nil {
		http.Error(w, "Failed to update client", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(client)
}

// DeleteClient handles DELETE /client/:client_id
func (h *ClientHandler) DeleteClient(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["client_id"]

	if err := h.service.DeleteClient(clientID); err != nil {
		http.Error(w, "Failed to delete client", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// RegisterClientRoutes registers all client routes
func RegisterClientRoutes(router *mux.Router, handler *ClientHandler) {
	router.HandleFunc("/client", handler.CreateClient).Methods("POST")
	router.HandleFunc("/client", handler.GetAllClients).Methods("GET")
	router.HandleFunc("/client/{client_id}", handler.GetClientByID).Methods("GET")
	router.HandleFunc("/client/{client_id}", handler.UpdateClient).Methods("PUT")
	router.HandleFunc("/client/{client_id}", handler.DeleteClient).Methods("DELETE")
}
