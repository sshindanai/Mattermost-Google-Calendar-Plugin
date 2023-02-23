package service

import (
	"fmt"

	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models/dbmodel"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/repository"
)

type ConnectStateService interface {
	Create(userID string, state string) error
	Get(userID string) (string, error)
	Delete(userID string) error
}

type connectStateService struct {
	connectStateRepository repository.ConnectStateRepository
}

func NewConnectStateService(connectStateRepository repository.ConnectStateRepository) ConnectStateService {
	return &connectStateService{
		connectStateRepository: connectStateRepository,
	}
}

func (c *connectStateService) Create(userID string, state string) error {
	if err := c.connectStateRepository.Insert(dbmodel.ConnectStates{
		UserID: userID,
		State:  state,
	}); err != nil {
		fmt.Printf("Error when insert connect state: %v", err)
		return err
	}
	return nil
}

func (c *connectStateService) Get(userID string) (string, error) {
	state, err := c.connectStateRepository.Get(userID)
	if err != nil {
		return "", err
	}
	return state, nil
}

func (c *connectStateService) Delete(userID string) error {
	if err := c.connectStateRepository.Delete(userID); err != nil {
		return err
	}
	return nil
}
