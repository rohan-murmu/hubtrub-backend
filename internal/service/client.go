package service

import (
	"github.com/scythrine05/hubtrub-server/internal/model"
	"github.com/scythrine05/hubtrub-server/internal/repository"
)

// ClientService handles client business logic
type ClientService struct {
	repo repository.ClientRepository
}

// NewClientService creates a new client service
func NewClientService(repo repository.ClientRepository) *ClientService {
	return &ClientService{repo: repo}
}

// GetAllClients retrieves all clients
func (s *ClientService) GetAllClients() ([]model.Client, error) {
	return s.repo.GetAllClients()
}

// GetClientByID retrieves a client by ID
func (s *ClientService) GetClientByID(clientID string) (*model.Client, error) {
	return s.repo.GetClientByID(clientID)
}

// GetClientByUserName retrieves a client by username
func (s *ClientService) GetClientByUserName(userName string) (*model.Client, error) {
	return s.repo.GetClientByUserName(userName)
}

// CreateClient creates a new client
func (s *ClientService) CreateClient(client *model.Client) error {
	return s.repo.CreateClient(client)
}

// UpdateClient updates a client
func (s *ClientService) UpdateClient(clientID string, client *model.Client) error {
	return s.repo.UpdateClient(clientID, client)
}

// DeleteClient deletes a client
func (s *ClientService) DeleteClient(clientID string) error {
	return s.repo.DeleteClient(clientID)
}
