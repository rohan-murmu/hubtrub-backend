package repository

import (
	"github.com/scythrine05/hubtrub-server/internal/model"
	"github.com/scythrine05/hubtrub-server/internal/util"
)

// FileClientRepository implements ClientRepository using file storage
type FileClientRepository struct {
	fr *util.FileRepository
}

// NewFileClientRepository creates a new file-based client repository
func NewFileClientRepository(filePath string) ClientRepository {
	return &FileClientRepository{
		fr: util.NewFileRepository(filePath),
	}
}

// GetAllClients returns all clients from file
func (r *FileClientRepository) GetAllClients() ([]model.Client, error) {
	var clients []model.Client
	err := r.fr.ReadAll(&clients)
	return clients, err
}

// GetClientByID returns a specific client by ID
func (r *FileClientRepository) GetClientByID(clientID string) (*model.Client, error) {
	var client model.Client
	err := r.fr.ReadByID(clientID, "clientId", &client)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

// GetClientByUserName returns a specific client by username
func (r *FileClientRepository) GetClientByUserName(userName string) (*model.Client, error) {
	var client model.Client
	err := r.fr.ReadByID(userName, "clientUserName", &client)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

// CreateClient creates a new client
func (r *FileClientRepository) CreateClient(client *model.Client) error {
	return r.fr.Write(client)
}

// UpdateClient updates an existing client
func (r *FileClientRepository) UpdateClient(clientID string, client *model.Client) error {
	return r.fr.Update(clientID, "clientId", client)
}

// DeleteClient deletes a client by ID
func (r *FileClientRepository) DeleteClient(clientID string) error {
	return r.fr.Delete(clientID, "clientId")
}
