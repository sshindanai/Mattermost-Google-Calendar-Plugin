package repository

import (
	"fmt"

	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models/dbmodel"
	"gorm.io/gorm"
)

type ConnectStateRepository interface {
	Insert(state dbmodel.ConnectStates) error
	Get(userID string) (string, error)
	Delete(userID string) error
}

type connectStateRepository struct {
	db *gorm.DB
}

func NewConnectStateRepository(db *gorm.DB) ConnectStateRepository {
	return &connectStateRepository{
		db: db,
	}
}

func (c *connectStateRepository) Insert(state dbmodel.ConnectStates) error {
	// insert state
	if err := c.db.Create(&state).Error; err != nil {
		fmt.Printf("error create connect state: %v", err)
		return err
	}
	return nil
}

func (c *connectStateRepository) Get(userID string) (string, error) {
	var stateResponse dbmodel.ConnectStates
	if err := c.db.Model(&dbmodel.ConnectStates{}).Where("user_id = ?", userID).First(&stateResponse).Error; err != nil {
		return "", err
	}
	return stateResponse.State, nil
}

func (c *connectStateRepository) Delete(userID string) error {
	if err := c.db.Where("user_id = ?", userID).Delete(&dbmodel.ConnectStates{}).Error; err != nil {
		return err
	}
	return nil
}
